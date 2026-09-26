# Перенос дизайна

Источник — предоставленный `max-miniapp-design-main.zip`, прочитан из Downloads; reference извлечён во временную папку, demo JS не импортирован. Рабочий backend: монорепозиторий на `50fcc75`, а не архив другой версии.

Сохранены Manrope, фон #F5F6F8, белые карточки, зелёный #4D8B63, тёмная основная кнопка, округления, мобильная ширина 430px, карточка дома, нижние пять вкладок. Исходный `assets/css/style.css` перенесён в `src/app/styles/reference.css`; глобальные селекторы header/nav/main ограничены классами React. `app.css` адаптирует формы, focus, ошибки, safe areas и реальные данные. Шрифт хранится в bundle, иконки — lucide-react; внешняя runtime-загрузка иконок отсутствует.

| Исходный HTML | React route / компонент | REST `/api/v1` |
| --- | --- | --- |
| `index.html` | `/` HomePage | GET /me, /issues, /announcements |
| `pages/problems/index.html` | `/issues` IssuesPage | GET /issues?page_size&page_token&status |
| `pages/create/index.html` | `/issues/new` CreateIssuePage | POST /uploads → прямой S3 PUT → POST /uploads/{id}/complete → POST /issues |
| `pages/issue/index.html` | `/issues/:issueId` IssueDetailsPage | GET /issues/{id}, /attachments/{id}/download-url; POST /issues/{id}/confirm |
| `pages/news/index.html` | `/news` NewsPage | GET /announcements |
| `pages/community/index.html` | `/community` CommunityPage | Доступная ссылка на объявления; прочие функции недоступны |
| `pages/profile/index.html` | `/profile` ProfilePage | GET /me; POST /session/active-house, /session/logout |
| `pages/chairman/index.html` | `/chairman` IssuesPage | GET /chairman/issues |
| `pages/chairman/issue.html` | `/chairman/issues/:issueId` IssueDetailsPage | GET/POST /issues/{id}/statement; PATCH /issues/{id}/status |
| `pages/chairman/announcement.html` | `/chairman/announcements/new` CreateAnnouncementPage | POST /announcements |
| `pages/chairman/residents.html`, `resident.html`, `logs.html`, `settings.html` | Не опубликованы | Нет маршрутов Gateway |

Удалены production demo-данные `window.MOCK_DATA`, render-функции и изменения домового состояния через DOM/innerHTML. Переходы выполняет React Router. Реальные запросы создаются только типизированным API-клиентом; нет запасных фальшивых объявлений при 503.

Не перенесены статистика без endpoint, назначение исполнителя, сроки, приоритеты, редактирование жителей, администратора и настроек дома. Опросы, календарь и инициативы подключены к Gateway и Community: формы создания, участие, результаты и управление. Нет элементов, которые показывают успешное голосование или публикацию без сервера.

Статусы переименованы по фактическому enum Issue, категории — по пяти контрактным значениям. Подтверждения показывают реальный счётчик, кнопка скрывает возможность подтверждения автором/повторно/после решения. Заявление всегда приходит с сервера; frontend не генерирует обращение и не отправляет его в УК.

Фото, таймлайн, тексты и версия заявления используются из DTO. Все тексты выводятся React как текст, никакого HTML от backend. Loading/empty/error/retry не заменяются демонстрационным контентом. Мобильная навигация и формы проверены в Chromium/WebKit; реальные системные клавиатуры MAX пока не проверялись.
