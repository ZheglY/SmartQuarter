"""Community participation and management contracts."""
def register(S,route,paths,ref,obj,string,array):
 uuid=string(format='uuid');date=string(format='date-time');number={'type':'integer','minimum':0}
 S['PollOption']=obj(dict(id=uuid,text=string(),position=number))
 S['Poll']=obj(dict(id=uuid,house_id=uuid,author_user_id=uuid,question=string(),status=string(enum=['POLL_STATUS_OPEN','POLL_STATUS_CLOSED']),options=array(ref('PollOption')),ends_at=date,created_at=date))
 S['PollDetails']=obj(dict(poll=ref('Poll'),results=array(obj(dict(option_id=uuid,votes_count=number))),total_votes=number,my_option_id=string()))
 S['PollList']=obj(dict(items=array(ref('Poll')),next_page_token=string()))
 S['CreatePoll']=obj(dict(question=string(minLength=1,maxLength=500),options=dict(array(string(minLength=1,maxLength=200)),minItems=2,maxItems=10,uniqueItems=True),ends_at=date))
 event=dict(title=string(minLength=1,maxLength=200),description=string(maxLength=5000),starts_at=date,ends_at=date)
 S['CalendarInput']=obj(event)
 S['CalendarEvent']=obj(dict(event,id=uuid,house_id=uuid,created_by=uuid,created_at=date))
 S['CalendarList']=obj(dict(items=array(ref('CalendarEvent'))))
 initiative=dict(title=string(minLength=1,maxLength=200),description=string(minLength=1,maxLength=5000))
 S['InitiativeInput']=obj(initiative)
 S['Initiative']=obj(dict(initiative,id=uuid,house_id=uuid,author_user_id=uuid,status=string(enum=['INITIATIVE_STATUS_OPEN','INITIATIVE_STATUS_CLOSED']),supports_count=number,supported_by_me={'type':'boolean'},created_at=date,updated_at=date))
 S['InitiativeList']=obj(dict(items=array(ref('Initiative')),next_page_token=string()))
 for method,path,response,body,manager,rpc in [
 ('get','/polls','PollList',None,False,'ListPolls'),('post','/polls','Poll','CreatePoll',True,'CreatePoll'),
 ('get','/polls/{id}','PollDetails',None,False,'GetPoll'),('post','/polls/{id}/close','PollDetails','Empty',True,'ClosePoll'),
 ('get','/calendar','CalendarList',None,False,'ListCalendarEvents'),('post','/calendar','CalendarEvent','CalendarInput',True,'CreateCalendarEvent'),('patch','/calendar/{id}','CalendarEvent','CalendarInput',True,'UpdateCalendarEvent'),
 ('get','/initiatives','InitiativeList',None,False,'ListInitiatives'),('post','/initiatives','Initiative','InitiativeInput',False,'CreateInitiative'),('post','/initiatives/{id}/close','Initiative','Empty',True,'CloseInitiative')]:
  route(method,'/api/v1'+path,rpc,ref(response),ref(body) if body else None,201 if rpc.startswith('Create') else 200,role='Current ACTIVE CHAIRMAN or house ADMIN' if manager else 'Current ACTIVE member',rpc='Community.'+rpc,idem=method!='get',pagination=rpc in ['ListPolls','ListInitiatives'])
 route('post','/api/v1/polls/{id}/vote','Vote once for an open poll',obj(dict(poll_id=uuid,option_id=uuid,total_votes=number,my_option_id=uuid)),obj(dict(option_id=uuid)),rpc='Community.VotePoll',idem=True)
 route('delete','/api/v1/calendar/{id}','Delete calendar event',obj(dict(id=uuid)),ref('Empty'),role='Current ACTIVE CHAIRMAN or house ADMIN',rpc='Community.DeleteCalendarEvent',idem=True)
 route('post','/api/v1/initiatives/{id}/support','Support an open initiative once',obj(dict(initiative_id=uuid,supports_count=number,supported_by_me={'type':'boolean'})),ref('Empty'),rpc='Community.SupportInitiative',idem=True)
 paths['/api/v1/polls']['get']['parameters'].append(dict(name='status',**{'in':'query'},schema=string(enum=['OPEN','CLOSED'])))
 for n in ['from','to']:paths['/api/v1/calendar']['get']['parameters'].append(dict(name=n,**{'in':'query'},required=True,schema=date,description='Inclusive start / exclusive end; events overlapping interval, up to 366 days.'))
 route('get','/api/v1/health','Application dependency readiness',obj(dict(status=string(enum=['ready']))),public=True)
