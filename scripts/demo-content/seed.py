"""Replace business content of ONE existing house; never write to Identity.

Run on the server from the repository root after stopping application services.
Requires verified PostgreSQL backups and the JSON manifest from demo-upload.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import uuid

ROOT = Path(__file__).resolve().parents[2]
COMPOSE = ['docker', 'compose', '-f', str(ROOT / 'deploy/server/compose.yaml')]

def command(args, data=None):
    return subprocess.run(args, input=data, text=True, encoding='utf-8', check=True, capture_output=True).stdout

def sql(db, statement):
    return command(COMPOSE + ['exec', '-T', db + '-db', 'psql', '-X', '-U', db, '-d', db + '_db', '-v', 'ON_ERROR_STOP=1', '-At'], statement)

def q(value):
    if value is None:
        return 'NULL'
    if isinstance(value, bool):
        return 'TRUE' if value else 'FALSE'
    if isinstance(value, (int, float)):
        return str(value)
    return "'" + str(value).replace("'", "''") + "'"

def insert(table, **row):
    return 'INSERT INTO ' + table + '(' + ','.join(row) + ') VALUES(' + ','.join(q(v) for v in row.values()) + ');'

def fingerprint():
    tables = sql('identity', "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;").splitlines()
    content = []
    for name in tables:
        content.append(name + sql('identity', 'SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),\'[]\'::jsonb) FROM "' + name.replace('"', '""') + '" t;'))
    return hashlib.sha256('\n'.join(content).encode()).hexdigest()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--house', required=True)
    parser.add_argument('--manifest', type=Path, required=True)
    parser.add_argument('--backup-dir', type=Path, required=True)
    parser.add_argument('--apply', action='store_true', help='Replace the selected house content; default only validates via rollback')
    args = parser.parse_args()
    house = str(uuid.UUID(args.house))
    namespace = uuid.UUID(house)
    uid = lambda name: str(uuid.uuid5(namespace, 'demo-content-v1/' + name))
    for service in ['identity-service', 'issue-service', 'community-service', 'max-gateway']:
        cid = command(COMPOSE + ['ps', '-a', '-q', service]).strip()
        if not cid or command(['docker', 'inspect', '-f', '{{.State.Running}}', cid]).strip() != 'false':
            raise RuntimeError('Stop application service first: ' + service)
    args.backup_dir = args.backup_dir.resolve(strict=True)
    for db in ['identity', 'issue', 'community']:
        path = args.backup_dir / (db + '.dump')
        if not path.is_file() or path.stat().st_size == 0:
            raise RuntimeError('Missing backup: ' + str(path))
        with path.open('rb') as stream:
            subprocess.run(COMPOSE + ['exec', '-T', db + '-db', 'pg_restore', '--list'], stdin=stream, stdout=subprocess.DEVNULL, check=True)
    before = fingerprint()
    rows = json.loads(sql('identity', 'SELECT COALESCE(jsonb_agg(to_jsonb(x)),\'[]\') FROM (SELECT m.user_id,m.role,h.address FROM memberships m JOIN houses h ON h.id=m.house_id WHERE m.house_id=' + q(house) + " AND m.status='ACTIVE' ORDER BY m.user_id) x;"))
    chairmen = [r for r in rows if r['role'] == 'CHAIRMAN']
    residents = [r['user_id'] for r in rows if r['role'] == 'RESIDENT']
    if len(chairmen) != 1 or not residents:
        raise RuntimeError('Expected an existing chairman and at least one resident; no users will be created')
    chairman, address = chairmen[0]['user_id'], chairmen[0]['address']
    author = residents[0]
    neighbor = residents[-1] if len(residents) > 1 else chairman
    images = {r['filename']: r for r in json.loads(args.manifest.read_text())}
    now = sql('issue', 'SELECT now();').strip()
    issue_sql = ["SET LOCAL standard_conforming_strings=on;"]
    scope = 'SELECT id FROM issues WHERE house_id=' + q(house)
    for table in ['confirmations', 'timeline_events', 'statement_drafts']:
        issue_sql.append('DELETE FROM ' + table + ' WHERE issue_id IN (' + scope + ');')
    issue_sql += ['DELETE FROM attachments WHERE house_id=' + q(house) + ';', 'DELETE FROM outbox_events WHERE payload->>\'house_id\'=' + q(house) + ';', 'DELETE FROM issues WHERE house_id=' + q(house) + ';']
    cases = [
        ('roaches', 'CLEANLINESS', 'Незваный сосед уже распаковал чемодан', 'Вечером у мусоропровода появились тараканы. Просим проверить подвал и провести обработку общих зон. Пока гость не попросил прописку!', 'Подъезд №1, первый этаж', 'DETECTED', 'uninvited-neighbor.png', 0),
        ('water', 'UTILITIES', 'В холле открылась утиная регата', 'Под радиатором у входа собирается вода. Нужны осмотр соединения, устранение течи и просушка пола. Резиновая утка довольна, жители — не очень.', 'Входная группа, подъезд №1', 'WAITING_RESULT', 'lobby-regatta.png', 1),
        ('bench', 'INFRASTRUCTURE', 'Скамейка снова держит слово', 'Расшатавшуюся доску у детской площадки закрепили, крепёж заменили. Теперь можно спокойно обсудить новости двора. Спасибо всем, кто сообщил!', 'Двор, скамейка у площадки', 'RESOLVED', 'bench-inspection.png', 1),
        ('lamp', 'SAFETY', 'Лестница играет в прятки', 'Между вторым и третьим этажами не включается свет. Просим заменить лампу и проверить датчик движения.', 'Подъезд №1, лестничный пролёт', 'READY_FOR_APPEAL', None, 1),
    ]
    marker = '\n\nДемонстрационный пример: ситуация и иллюстрация вымышлены.'
    for name, category, title, body, location, status, filename, count in cases:
        issue_id = uid('issue/' + name)
        issue_sql.append(insert('issues', id=issue_id, house_id=house, created_by=author, house_address_snapshot=address, category=category, description=title+'\n\n'+body+marker, location_text=location, status=status, confirmations_count=count, created_at=now, updated_at=now, resolved_at=now if status == 'RESOLVED' else None))
        issue_sql.append(insert('timeline_events', id=uid('created/'+name), issue_id=issue_id, type='issue.created', actor_user_id=author, payload='{}', created_at=now))
        if count:
            issue_sql.append(insert('confirmations', issue_id=issue_id, user_id=neighbor, created_at=now))
            issue_sql.append(insert('timeline_events', id=uid('confirmed/'+name), issue_id=issue_id, type='issue.confirmed', actor_user_id=neighbor, payload='{}', created_at=now))
        if status != 'DETECTED':
            issue_sql.append(insert('timeline_events', id=uid('status/'+name), issue_id=issue_id, type='issue.status_changed', actor_user_id=chairman, payload=json.dumps({'from':'DETECTED','to':status}), created_at=now))
        if filename:
            image = images[filename]
            issue_sql.append(insert('attachments', id=image['id'], house_id=house, issue_id=issue_id, uploaded_by=author, object_key=image['object_key'], original_filename=filename, mime_type='image/png', size_bytes=image['size_bytes'], sha256=image['sha256'], etag=image['etag'], status='ATTACHED', upload_expires_at=now, created_at=now, updated_at=now))
        if name == 'water':
            issue_sql.append(insert('statement_drafts', id=uid('statement/water'), issue_id=issue_id, version=1, status='DRAFT', body='Демонстрационный черновик обращения\n\nПросим устранить течь радиатора во входной группе по адресу: '+address+'. Просим сообщить срок выполнения работ и результат осмотра.\n\nОбразец не отправлен в управляющую организацию.', chairman_note='Образец для проверки просмотра и копирования заявления.', source_snapshot=json.dumps({'issue_id':issue_id,'house_id':house,'demo':True}), created_by=chairman, created_at=now, updated_at=now))
            issue_sql.append(insert('timeline_events', id=uid('statement-event/water'), issue_id=issue_id, type='statement.generated', actor_user_id=chairman, payload='{}', created_at=now))
    community_sql = ["SET LOCAL standard_conforming_strings=on;"]
    for table in ['announcements', 'polls', 'calendar_events', 'initiatives', 'service_contacts', 'service_contact_audit']:
        community_sql.append('DELETE FROM ' + table + ' WHERE house_id=' + q(house) + ';')
    community_sql.append('DELETE FROM outbox_events WHERE payload->>\'house_id\'=' + q(house) + ';')
    announcements = [
        ('cleanup', 'Субботник без героизма: час для любимого двора', 'Встречаемся у входа в субботу в 11:00. Перчатки и мешки будут на месте. Можно прийти на 20 минут — каждый вклад важен. После уборки обменяемся идеями для клумб.'),
        ('water', 'Проверяем радиаторы до холодов', 'Посмотрите, нет ли капель у соединений радиаторов. Если заметили течь в общих зонах — создайте заявку с фотографией. Не пытайтесь самостоятельно перекрывать общедомовые коммуникации.'),
        ('welcome', 'Добро пожаловать в наш цифровой двор', 'Здесь можно сообщить о неисправности, поддержать идею соседей, проголосовать и посмотреть календарь. Полезные контакты находятся в профиле. Давайте сделаем заботу о доме привычным делом.'),
    ]
    for name, title, body in announcements:
        community_sql.append(insert('announcements', id=uid('announcement/'+name), house_id=house, author_user_id=chairman, title=title, body=body+'\n\nДемонстрационное объявление. Даты и работы не являются реальным уведомлением.', status='PUBLISHED', published_at=now, created_at=now))
    from datetime import datetime, timedelta
    base = datetime.fromisoformat(now)
    polls = [('flowers','Что посадим у входа?', ['Лаванду и злаки','Неприхотливые многолетники','Пока оставим газон'],'OPEN',7), ('meeting','Когда удобнее собраться соседям?', ['В будни после 19:00','В субботу утром','Онлайн вечером'],'OPEN',10), ('benches','Какой цвет выберем для скамеек?', ['Тёплый орех','Графит','Зелёный'],'CLOSED',-1)]
    for name, question, options, status, days in polls:
        poll_id = uid('poll/'+name)
        community_sql.append(insert('polls', id=poll_id, house_id=house, author_user_id=chairman, question=question+' · демо', status=status, ends_at=(base+timedelta(days=days)).isoformat(), created_at=now))
        for index, option in enumerate(options):
            community_sql.append(insert('poll_options', id=uid('option/'+name+'/'+str(index)), poll_id=poll_id, text=option, position=index))
        community_sql.append(insert('poll_votes', poll_id=poll_id, option_id=uid('option/'+name+'/0'), user_id=neighbor))
    for name, title, body, status in [('books','Книжная полка для соседей','Предлагаем поставить в холле небольшой стеллаж: принёс книгу — взял другую. Начнём с одной полки и аккуратных правил обмена.','OPEN'),('bikes','Велосипедам — своё место','Предлагаем обсудить велопарковку во дворе, чтобы проходы оставались свободными. Сначала соберём пожелания и варианты размещения.','OPEN'),('plants','Зелёный уголок у входа','Соседи договорились о неприхотливых растениях и графике ухода. Сбор предложений завершён, идея переходит к обсуждению исполнения.','CLOSED')]:
        initiative_id = uid('initiative/'+name)
        community_sql.append(insert('initiatives', id=initiative_id, house_id=house, author_user_id=author, title=title, description=body+'\n\nДемонстрационная инициатива; поддержка показана для примера.', status=status, supports_count=1, created_at=now, updated_at=now))
        community_sql.append(insert('initiative_supports', initiative_id=initiative_id, user_id=neighbor))
    for index, (title, body, day) in enumerate([('Час заботы о дворе','Лёгкая уборка и идеи для клумб. Сбор у первого подъезда.',1),('Встреча соседей','Обсудим освещение, велопарковку и книжный обмен.',3),('Проверка освещения','Демонстрационный обход общих зон: лестницы, вход и двор.',6)]):
        start = (base + timedelta(days=day)).replace(hour=15, minute=0, second=0, microsecond=0)
        community_sql.append(insert('calendar_events', id=uid('calendar/'+str(index)), house_id=house, created_by=chairman, title=title, description=body+'\n\nДемонстрационное событие. Реальная встреча не назначена.', starts_at=start.isoformat(), ends_at=(start+timedelta(hours=1)).isoformat(), created_at=now))
    for index, (category, title) in enumerate([('MANAGEMENT_COMPANY','Управляющая организация'),('EMERGENCY_DISPATCH','Диспетчерская дома'),('ELEVATOR','Обслуживание лифтов'),('WASTE','Вывоз отходов')]):
        contact_id = uid('contact/'+str(index))
        community_sql.append(insert('service_contacts', id=contact_id, house_id=house, category=category, title=title+' · образец', organization_name='Учебный справочник дома', phone='+7000000000'+str(index), email='service'+str(index)+'@example.com', website='https://example.com', description='Демонстрационный контакт. Номер нерабочий, звонить не нужно. Замените реквизиты проверенными данными своей организации.', emergency=False, sort_order=index, is_active=True, created_by=chairman))
        community_sql.append(insert('service_contact_audit', id=uid('contact-audit/'+str(index)), contact_id=contact_id, house_id=house, actor_user_id=chairman, operation='create'))
    plans = {'issue':'\n'.join(issue_sql), 'community':'\n'.join(community_sql)}
    for db, plan in plans.items():
        (args.backup_dir / ('demo-'+db+'.sql')).write_text('BEGIN;\n'+plan+'\nCOMMIT;\n', encoding='utf-8')
        sql(db, 'BEGIN;\n'+plan+'\nROLLBACK;')
    if fingerprint() != before:
        raise RuntimeError('Identity changed during preflight; aborting')
    if args.apply:
        for db, plan in plans.items():
            sql(db, 'BEGIN;\n'+plan+'\nCOMMIT;')
        if fingerprint() != before:
            raise RuntimeError('Identity changed during seed')
        # Remove only obsolete content notifications of this house; do not touch sessions or identity events.
        cursor, removed = '-', 0
        while True:
            entries = json.loads(command(COMPOSE+['exec','-T','redis','redis-cli','--json','XRANGE','stream:notifications',cursor,'+','COUNT','500']))
            if not entries: break
            for entry_id, values in entries:
                event = json.loads(dict(zip(values[::2],values[1::2])).get('data','{}'))
                if event.get('producer') in ['issue-service','community-service'] and event.get('payload',{}).get('house_id') == house:
                    command(COMPOSE+['exec','-T','redis','redis-cli','XDEL','stream:notifications',entry_id]); removed += 1
            cursor = '(' + entries[-1][0]
        print('Obsolete content notifications removed:', removed)
    (args.backup_dir/'identity-fingerprint.txt').write_text(before+'\n')
    print('APPLIED' if args.apply else 'VALIDATED (rolled back)', '4 issues, 3 images, 1 statement, 3 announcements, 3 polls, 3 events, 3 initiatives, 4 contacts. Identity unchanged.')

if __name__ == '__main__':
    main()
