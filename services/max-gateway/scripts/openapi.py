"""Generate the canonical OpenAPI for implemented Gateway routes. Python 3 only."""
import json
from pathlib import Path
S={}
def ref(name):return {'$ref':'#/components/schemas/'+name}
def obj(props,required=None):return {'type':'object','additionalProperties':False,'properties':props,'required':list(props) if required is None else required}
def string(**kw):return {'type':'string',**kw}
def array(items):return {'type':'array','items':items}
uuid=string(format='uuid');date=string(format='date-time');nullable_date={'type':['string','null'],'format':'date-time'}
S['IssueCategory']=string(enum=['SAFETY','CLEANLINESS','UTILITIES','INFRASTRUCTURE','OTHER'])
S['IssueStatus']=string(enum=['DETECTED','CONFIRMING','READY_FOR_APPEAL','HANDED_TO_CHAIRMAN','MARKED_SENT','WAITING_RESULT','RESOLVED'])
S['Issue']=obj(dict(id=uuid,house_id=uuid,created_by=uuid,house_address_snapshot=string(),category=ref('IssueCategory'),description=string(),location_text=string(),status=ref('IssueStatus'),confirmations_count={'type':'integer','format':'int32'},created_at=date,updated_at=date,resolved_at=nullable_date))
S['Attachment']=obj(dict(id=uuid,house_id=uuid,issue_id={'type':['string','null'],'format':'uuid'},uploaded_by=uuid,original_filename=string(),mime_type=string(),size_bytes={'type':'integer','format':'int64'},sha256=string(),etag=string(),status=string(enum=['UPLOADING','READY','ATTACHED','REJECTED','EXPIRED']),upload_expires_at=date,created_at=date,updated_at=date))
S['Statement']=obj(dict(id=uuid,issue_id=uuid,version={'type':'integer'},status=string(),body=string(),chairman_note=string(),source_snapshot={'type':'object','additionalProperties':True},created_by=uuid,created_at=date,updated_at=date))
S['TimelineEvent']=obj(dict(id=uuid,issue_id=uuid,type=string(),actor_user_id=uuid,payload={'type':'object','additionalProperties':True},created_at=date))
S['IssueDetails']=obj(dict(issue=ref('Issue'),attachments=array(ref('Attachment')),confirmed_by_me={'type':'boolean'},timeline=array(ref('TimelineEvent')),latest_statement={'oneOf':[ref('Statement'),{'type':'null'}]}))
S['User']=obj(dict(id=uuid,max_user_id=string(pattern='^[0-9]+$',description='Decimal int64 encoded as a JSON string.'),display_name=string(),username=string(),created_at=date,updated_at=date))
S['House']=obj(dict(id=uuid,name=string(),address=string(),city=string()))
S['Membership']=obj(dict(id=uuid,user_id=uuid,house_id=uuid,role=string(enum=['RESIDENT','CHAIRMAN','ADMIN']),status=string(enum=['ACTIVE','INACTIVE'])))
S['UserContext']=obj(dict(user=ref('User'),houses=array(ref('House')),memberships=array(ref('Membership')),default_house_id=string(),active_house_id=string()))
S['Session']=obj(dict(user_context=ref('UserContext'),expires_at=date))
S['Error']=obj({'error':obj(dict(code=string(),message=string(),request_id=uuid))})
S['UploadRequest']=obj(dict(filename=string(minLength=1,maxLength=255),mime_type=string(enum=['image/png','image/jpeg']),size_bytes={'type':'integer','minimum':1,'maximum':10485760}))
S['Upload']=obj(dict(upload_id=uuid,presigned_url=string(format='uri'),expires_at=date,required_headers={'type':'object','additionalProperties':{'type':'string'},'examples':[{'Content-Type':'image/png','If-None-Match':'*'}]}))
S['DownloadURL']=obj(dict(url=string(format='uri'),expires_at=date))
S['CreateIssue']=obj(dict(category=ref('IssueCategory'),description=string(minLength=1),location_text=string(),attachment_ids=array(uuid)),['category','description','attachment_ids'])
S['IssueList']=obj(dict(items=array(ref('Issue')),next_page_token=string()))
S['Confirmation']=obj(dict(confirmation_count={'type':'integer'},confirmed_by_me={'type':'boolean'}))
S['Announcement']=obj(dict(id=uuid,house_id=uuid,author_user_id=uuid,title=string(),body=string(),status=string(enum=['PUBLISHED']),published_at=date,created_at=date))
S['AnnouncementList']=obj(dict(items=array(ref('Announcement')),next_page_token=string()))
S['CreateAnnouncement']=obj(dict(title=string(minLength=1,maxLength=200),body=string(minLength=1,maxLength=10000)))
S['Empty']=obj({})
S['MaxUpdate']={'type':'object','required':['update_type','timestamp'],'properties':{'update_type':string(),'timestamp':{'type':'integer','format':'int64'},'user':{'type':'object'},'message':{'type':'object'},'callback':{'type':'object'}},'additionalProperties':True,'description':'Official MAX Update (not the internal event envelope). Supports bot_started, /start message_created, message_callback. Unknown types are acknowledged.'}
paths={}
def route(method,path,title,response=None,request=None,code=200,role='ACTIVE member',rpc=None,idem=False,public=False,pagination=False,example=None):
 op={'summary':title,'operationId':method+'_'+path.replace('/','_').replace('{','').replace('}',''),'description':('Required role: '+role+'. ' if not public else '')+(('gRPC: '+rpc+'. ') if rpc else ''),'responses':{},'parameters':[{'name':'X-Request-Id','in':'header','required':False,'schema':uuid}]}
 if public:op['security']=[]
 if '{id}' in path:op['parameters'].append({'name':'id','in':'path','required':True,'schema':uuid})
 if method in ('post','patch') and path!='/webhooks/max':op['parameters'].append({'name':'Origin','in':'header','required':True,'schema':string(format='uri'),'description':'Exact origin from TRUSTED_ORIGINS. Missing or foreign Origin is rejected (CSRF protection).'})
 if idem:op['parameters'].append({'name':'Idempotency-Key','in':'header','schema':uuid,'description':'Optional. Scope: user + active house. Key binds method, path and compact JSON bytes (key order significant). Same request replays success for 24h; changed payload 409 IDEMPOTENCY_KEY_REUSED; concurrent/ambiguous outcome 409 IDEMPOTENCY_IN_PROGRESS. Reconcile before creating a new key; no exactly-once guarantee.'})
 if pagination:op['parameters'] += [{'name':'page_size','in':'query','schema':{'type':'integer','default':20,'minimum':1,'maximum':100}},{'name':'page_token','in':'query','schema':string(),'description':'Opaque token; empty next_page_token means end.'}]
 if method=='get' and path in ['/api/v1/issues','/api/v1/chairman/issues']:op['parameters'].append({'name':'status','in':'query','schema':array(ref('IssueStatus')),'style':'form','explode':True,'description':'Repeated status parameter; comma-separated values also accepted.'})
 if request:op['requestBody']={'required':True,'content':{'application/json':{'schema':request,**({'example':example} if example else {})}}}
 success={'description':'Success','headers':{'X-Request-Id':{'schema':uuid}}}
 if response:success['content']={'application/json':{'schema':response}}
 op['responses'][str(code)]=success
 for c,desc in [(400,'Invalid JSON, enum, identifier or pagination'),(401,'Missing/invalid/expired session or initData'),(403,'Forbidden membership, role or Origin'),(404,'Resource not found'),(409,'Conflict, invalid transition or idempotency reservation'),(413,'Body exceeds 256 KiB'),(415,'application/json required'),(429,'Rate limit exceeded'),(500,'Internal error'),(502,'Invalid upstream response'),(503,'Dependency unavailable'),(504,'Deadline exceeded'),(408,'Request canceled')]:op['responses'][str(c)]={'description':desc,'content':{'application/json':{'schema':ref('Error')}}}
 paths.setdefault(path,{})[method]=op
route('post','/api/v1/session/max','Validate MAX and create Redis session',ref('Session'),obj({'init_data':string()}),public=True,rpc='Identity.UpsertMaxUser -> GetUserContext -> GetMembership',example={'init_data':'<raw MAX initData>'})
paths['/api/v1/session/max']['post']['description']+='BLOCKED in production until identity.proto is agreed; test-only Identity adapter is excluded from app. Cookie is HttpOnly, Path=/; production Secure + SameSite=None; local Lax.'
route('get','/api/v1/me','Get user context',ref('UserContext'),role='Any session',rpc='Identity.GetUserContext')
route('post','/api/v1/session/active-house','Select active membership',obj(dict(active_house_id=uuid,role=string())),obj({'house_id':uuid}),role='ACTIVE membership of selected house',rpc='Identity.GetMembership')
route('post','/api/v1/session/logout','Revoke session',request=ref('Empty'),code=204,role='Any session')
route('post','/api/v1/uploads','Create direct S3 upload',ref('Upload'),ref('UploadRequest'),201,rpc='Issue.CreateUpload',example={'filename':'photo.png','mime_type':'image/png','size_bytes':1234})
route('post','/api/v1/uploads/{id}/complete','Verify uploaded object',ref('Attachment'),ref('Empty'),rpc='Issue.CompleteUpload')
route('get','/api/v1/attachments/{id}/download-url','Get signed private download URL',ref('DownloadURL'),rpc='Issue.GetAttachmentDownloadURL')
route('post','/api/v1/issues','Create issue',ref('Issue'),ref('CreateIssue'),201,rpc='Issue.CreateIssue',idem=True,example={'category':'INFRASTRUCTURE','description':'Broken lighting','location_text':'Entrance 1','attachment_ids':['55555555-5555-4555-8555-555555555555']})
route('get','/api/v1/issues','List house issues',ref('IssueList'),rpc='Issue.ListIssues',pagination=True)
route('get','/api/v1/issues/{id}','Get issue details',ref('IssueDetails'),rpc='Issue.GetIssue')
route('post','/api/v1/issues/{id}/confirm','Confirm neighbor issue',ref('Confirmation'),ref('Empty'),rpc='Issue.ConfirmIssue')
route('patch','/api/v1/issues/{id}/status','Change issue status',ref('Issue'),obj({'new_status':ref('IssueStatus')}),role='CHAIRMAN or ADMIN',rpc='Issue.UpdateIssueStatus')
route('post','/api/v1/issues/{id}/statement','Generate next statement version',ref('Statement'),obj({'chairman_note':string()},[]),201,role='CHAIRMAN or ADMIN',rpc='Issue.GenerateStatement',idem=True)
route('get','/api/v1/issues/{id}/statement','Get latest statement',ref('Statement'),role='CHAIRMAN or ADMIN',rpc='Issue.GetStatement')
route('get','/api/v1/chairman/issues','List unresolved chairman queue',ref('IssueList'),role='CHAIRMAN or ADMIN',rpc='Issue.ListIssues(chairman_queue=true)',pagination=True)
route('post','/api/v1/announcements','Publish announcement',ref('Announcement'),ref('CreateAnnouncement'),201,role='CHAIRMAN or ADMIN',rpc='Community.CreateAnnouncement',idem=True)
route('get','/api/v1/announcements','List announcements',ref('AnnouncementList'),rpc='Community.ListAnnouncements',pagination=True)
for method in ['post','get']:paths['/api/v1/announcements'][method]['description']+='Transport contract tested with a fake gRPC endpoint; real Community runtime integration NOT VERIFIED. Missing configuration returns 503.'
route('post','/webhooks/max','Receive official MAX Update',obj({'ok':{'type':'boolean'}}),ref('MaxUpdate'),public=True,example={'update_type':'bot_started','timestamp':1771409719000,'user':{'user_id':123456789,'is_bot':False}})
paths['/webhooks/max']['post']['security']=[{'MaxWebhookSecret':[]}]
for path in ['/livez','/healthz','/readyz']:route('get',path,'Process liveness' if path!='/readyz' else 'Redis, Identity, Issue gRPC and Issue dependencies readiness',obj({'status':string()}),public=True)
paths['/metrics']={'get':{'summary':'Prometheus metrics; restrict to internal monitoring at ingress','security':[],'responses':{'200':{'description':'Prometheus exposition','content':{'text/plain':{'schema':string()}}}}}}
spec={'openapi':'3.1.0','info':{'title':'SmartQuarter max-gateway','version':'1.0.0','description':'Implemented Gateway API. Server owns actor user/house/role. No multipart uploads. Identity production integration is blocked pending agreed proto; Community announcements have transport coverage only. gRPC v1 exposes status codes without ErrorInfo, therefore stable generic HTTP error codes are used rather than parsing messages.'},'servers':[{'url':'http://localhost:18080'}],'security':[{'SessionAuth':[]}],'paths':paths,'components':{'securitySchemes':{'SessionAuth':{'type':'apiKey','in':'cookie','name':'sq_session'},'MaxWebhookSecret':{'type':'apiKey','in':'header','name':'X-Max-Bot-Api-Secret'}},'schemas':S}}
# Hoist common error responses to keep the canonical contract small.
responses={}
for path in paths.values():
 for op in path.values():
  for code,res in list(op['responses'].items()):
   if code.isdigit() and int(code)>=400:
    name='Error'+code;responses[name]=res;op['responses'][code]={'$ref':'#/components/responses/'+name}
spec['components']['responses']=responses
# YAML emitter for this JSON-compatible schema; no build dependency on PyYAML.
def scalar(v):return json.dumps(v,ensure_ascii=False)
def yaml(v,indent=0):
 pad=' '*indent
 if isinstance(v,dict):
  if not v:return pad+'{}\n'
  out=''
  for k,x in v.items():
   if isinstance(x,(dict,list)) and x:out+=pad+scalar(k)+':\n'+yaml(x,indent+2)
   else:out+=pad+scalar(k)+': '+scalar(x)+'\n'
  return out
 if isinstance(v,list):
  out=''
  for x in v:
   if isinstance(x,(dict,list)) and x:out+=pad+'-\n'+yaml(x,indent+2)
   else:out+=pad+'- '+scalar(x)+'\n'
  return out
 return pad+scalar(v)+'\n'
root=Path(__file__).resolve().parents[3]
(root/'contracts/openapi/openapi.yaml').write_text(yaml(spec),encoding='utf-8')

