BEGIN;
CREATE TABLE IF NOT EXISTS service_contacts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),house_id uuid NOT NULL,
 category text NOT NULL CHECK(category IN ('MANAGEMENT_COMPANY','HOA','EMERGENCY_DISPATCH','ELECTRICITY','WATER','HEATING','GAS','ELEVATOR','WASTE','INTERNET','SECURITY','OTHER')),
 title varchar(150) NOT NULL,organization_name varchar(255) NOT NULL DEFAULT '',
 phone varchar(20) NOT NULL,additional_phone varchar(20) NOT NULL DEFAULT '',email varchar(254) NOT NULL DEFAULT '',
 website varchar(2048) NOT NULL DEFAULT '',description varchar(2000) NOT NULL DEFAULT '',emergency boolean NOT NULL DEFAULT false,
 sort_order integer NOT NULL DEFAULT 0,is_active boolean NOT NULL DEFAULT true,created_by uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS service_contacts_house ON service_contacts(house_id,category,sort_order) WHERE is_active;
CREATE TABLE IF NOT EXISTS service_contact_audit(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),contact_id uuid NOT NULL,house_id uuid NOT NULL,actor_user_id uuid NOT NULL,operation text NOT NULL,occurred_at timestamptz NOT NULL DEFAULT now());
COMMIT;
