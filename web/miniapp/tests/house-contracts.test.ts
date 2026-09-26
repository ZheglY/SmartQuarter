import { describe,it,expect } from 'vitest';
import { HouseInvitationSchema,NotificationPreferencesSchema,PlatformUserSchema,AdminHouseSchema } from '../src/shared/api/house';
import { contactInputSchema } from '../src/shared/api/contacts';
import { launchHouseRoute } from '../src/shared/max/bridge';
describe('house contracts',()=>{
 it('distinguishes decimal MAX IDs and optional chairman UUIDs',()=>{
  expect(PlatformUserSchema.safeParse({id:'11111111-1111-4111-8111-111111111111',display_name:'Anna',max_user_id:'224304993',can_register_house:true,managed_houses:0}).success).toBe(true);
  expect(AdminHouseSchema.safeParse({id:'11111111-1111-4111-8111-111111111111',name:'House',city:'City',address:'Street',chairman_user_id:'',chairman_display_name:''}).success).toBe(true);
 });
 it('accepts only known invitation states',()=>{
  const v={id:'11111111-1111-4111-8111-111111111111',house_id:'22222222-2222-4222-8222-222222222222',created_by:'33333333-3333-4333-8333-333333333333',max_uses:1,used_count:0,status:'INVITATION_STATUS_ACTIVE'};
  expect(HouseInvitationSchema.safeParse(v).success).toBe(true);expect(HouseInvitationSchema.safeParse({...v,status:'INVITATION_STATUS_UNKNOWN'}).success).toBe(false);
 });
 it('requires all notification preference switches',()=>{expect(NotificationPreferencesSchema.safeParse({notifications_enabled:false}).success).toBe(false);});
 it('rejects executable contact links',()=>{
  const v={category:'OTHER',title:'Служба',organization_name:'',phone:'+79991234567',additional_phone:'',email:'',website:'javascript:alert(1)',description:'',emergency:false,sort_order:0};expect(contactInputSchema.safeParse(v).success).toBe(false);
 });
 it('recognizes only bounded launch routes and invitation tokens',()=>{
  expect(launchHouseRoute('start_param=register_house')).toBe('/houses/register');
  expect(launchHouseRoute('start_param=invite_'+'a'.repeat(43))).toBe('/invitations/redeem?token='+'a'.repeat(43));
  expect(launchHouseRoute('start_param=invite_<script>')).toBeNull();expect(launchHouseRoute('start_param=https://evil.test')).toBeNull();
 });
});
