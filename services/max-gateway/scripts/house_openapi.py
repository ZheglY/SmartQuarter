"""House workflow OpenAPI extension, used by openapi.py."""
import json,re
from pathlib import Path
def register(S,route,paths,ref,obj,string,array):
 root=Path(__file__).resolve().parents[3]
 registry=json.loads((root/'contracts/house-workflow.json').read_text(encoding='utf-8'))
 for name,values in registry['enums'].items():
  prefix=re.sub(r'(?<!^)(?=[A-Z])','_',name).upper()
  S[name]=string(enum=[prefix+'_'+v for v in values])
 def field(t,f):
  if t=='string':
   if f=='chairman_user_id':return string(description='Chairman UUID or empty when vacant')
   if f=='max_user_id':return string(pattern='^[0-9]+$')
   if f=='id' or f.endswith('_user_id') or f=='house_id' or f=='created_by':return string(format='uuid')
   if f=='resulting_house_id':return string(description='Created house UUID; empty before approval')
   return string()
  if t=='bool':return {'type':'boolean'}
  if t=='int32':return {'type':'integer','format':'int32'}
  if t=='google.protobuf.Timestamp':return {'type':['string','null'],'format':'date-time'}
  if t.startswith('repeated '):return array(ref(t[9:]))
  return ref(t)
 for name,fields in registry['entities'].items():
  if name.startswith('NotificationRecipient'):continue
  S[name]=obj({f:field(t,f) for t,f in fields},[f for t,f in fields if t!='google.protobuf.Timestamp'])
 for o in registry['operations']:
  p='/api/v1'+o['path'];method=o['method'].lower();pf=re.findall(r'{(\w+)}',p)
  fields={f:field(t,f) for t,f in o['fields'] if f not in pf}
  for f in ['name','city','address','query','token','reason']:
   if f in fields:fields[f].update(maxLength={'name':255,'city':100,'address':1000,'query':255,'token':43,'reason':1000}[f])
  if 'expires_in_hours' in fields:fields['expires_in_hours'].update(minimum=1,maximum=168)
  if 'max_uses' in fields:fields['max_uses'].update(minimum=1,maximum=100)
  role={'session':'Authenticated user, including without membership','chairman':'Administrator-approved chairman or platform administrator','manager':'Current ACTIVE CHAIRMAN or house ADMIN; Identity verifies scope','admin':'Explicit platform administrator allowlist'}[o['access']]
  route(method,p,o['name'],ref(o['response']),None if method=='get' else obj(fields,[f for f in fields if f not in ['reason','platform_admin_override']]),code=o['code'],role=role,rpc='Identity.HouseService.'+o['name'],idem=method!='get')
  if method=='get':paths[p][method]['parameters'] += [{'name':f,'in':'query','required':True,'schema':schema} for f,schema in fields.items()]
  paths[p][method]['description']+=' List responses are bounded to 100 records (members: 500). Mutation requests require a JSON object, including {} for commands without fields.'
 cats='MANAGEMENT_COMPANY HOA EMERGENCY_DISPATCH ELECTRICITY WATER HEATING GAS ELEVATOR WASTE INTERNET SECURITY OTHER'.split()
 props={k:string(maxLength=n) for k,n in [('title',150),('organization_name',255),('phone',16),('additional_phone',16),('email',254),('website',2048),('description',2000)]}
 props.update(category=string(enum=cats),emergency={'type':'boolean'},sort_order={'type':'integer','minimum':0,'maximum':10000})
 props['phone']['pattern']=r'^\+[1-9][0-9]{7,14}$'
 S['ServiceContactInput']=obj(props,['category','title','phone'])
 S['ServiceContact']=obj(dict(props,id=string(format='uuid'),house_id=string(format='uuid'),is_active={'type':'boolean'},created_by=string(),created_at={'type':['string','null'],'format':'date-time'},updated_at={'type':['string','null'],'format':'date-time'}),list(props)+['id','house_id','is_active'])
 S['ServiceContactList']=obj({'items':array(ref('ServiceContact'))})
 route('get','/api/v1/house/service-contacts','List directory contacts',ref('ServiceContactList'),rpc='Community.ListServiceContacts')
 paths['/api/v1/house/service-contacts']['get']['parameters'].append({'name':'include_archived','in':'query','schema':{'type':'boolean'},'description':'Managers only; residents always receive active contacts without audit fields.'})
 for method,code,rpc in [('post',201,'CreateServiceContact'),('patch',200,'UpdateServiceContact'),('delete',200,'ArchiveServiceContact')]:
  p='/api/v1/chairman/service-contacts'+('' if method=='post' else '/{id}')
  route(method,p,rpc,ref('ServiceContact'),ref('Empty' if method=='delete' else 'ServiceContactInput'),code,role='Current ACTIVE CHAIRMAN or ADMIN',rpc='Community.'+rpc,idem=True)
  if method=='patch':paths[p][method]['description']+=' Replaces the complete editable contact record; omitted optional fields are cleared.'
