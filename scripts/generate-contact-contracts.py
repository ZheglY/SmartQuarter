from pathlib import Path
root=Path(__file__).resolve().parents[1]
p=root/'contracts/proto/smartquarter/community/v1/community.proto'
src=p.read_text(encoding='utf-8').split('// Service contacts (additive contract).')[0].rstrip()
if 'rpc CreateServiceContact' not in src:
    src=src.replace('service CommunityService {','''service CommunityService {
  rpc CreateServiceContact(CreateServiceContactRequest) returns (ServiceContact);
  rpc ListServiceContacts(ListServiceContactsRequest) returns (ListServiceContactsResponse);
  rpc GetServiceContact(GetServiceContactRequest) returns (ServiceContact);
  rpc UpdateServiceContact(UpdateServiceContactRequest) returns (ServiceContact);
  rpc ArchiveServiceContact(ArchiveServiceContactRequest) returns (ServiceContact);''')
fields=[('string',x) for x in ['category','title','organization_name','phone','additional_phone','email','website','description']]+[('bool','emergency'),('int32','sort_order')]
def msg(n,fs):return 'message '+n+' {\n'+''.join(f'  {t} {f} = {i};\n' for i,(t,f) in enumerate(fs,1))+'}\n'
src+='\n\n// Service contacts (additive contract).\n'
src+=msg('ServiceContactInput',fields)
src+=msg('ServiceContact',[('string','id'),('string','house_id')]+fields+[('bool','is_active'),('string','created_by'),('google.protobuf.Timestamp','created_at'),('google.protobuf.Timestamp','updated_at')])
src+=msg('CreateServiceContactRequest',[('string','house_id'),('ServiceContactInput','contact')])
src+=msg('UpdateServiceContactRequest',[('string','house_id'),('string','id'),('ServiceContactInput','contact')])
for n in ['Get','Archive']:src+=msg(n+'ServiceContactRequest',[('string','house_id'),('string','id')])
src+=msg('ListServiceContactsRequest',[('string','house_id'),('bool','include_archived')])
src+=msg('ListServiceContactsResponse',[('repeated ServiceContact','items')])
p.write_text(src,encoding='utf-8')
