CREATE SCHEMA IF NOT EXISTS frontdesk;
CREATE TABLE IF NOT EXISTS frontdesk.users (
	id serial4 NOT NULL,
	username varchar(50) NOT NULL,
	password varchar(255) NOT NULL,
	CONSTRAINT users_pkey PRIMARY KEY (id)
);
----
CREATE TABLE IF NOT EXISTS frontdesk.activesessions (
	userid int4 NOT NULL,
	token varchar(255) NOT NULL,
	expire timestamp NOT NULL,
	CONSTRAINT activesessions_pkey PRIMARY KEY (token),
	CONSTRAINT activesessions_userid_fkey FOREIGN KEY (userid) REFERENCES frontdesk.users(id) ON DELETE CASCADE
);
----
CREATE TABLE IF NOT EXISTS frontdesk.cloudflare (
    config_id int4 NOT NULL DEFAULT 1,
    accountId varchar(255) NOT NULL,
    tunnelId varchar(255) NOT NULL,
    cloudflareAPIToken varchar(255) NOT NULL,
    localAddress varchar(255) NOT NULL,
    zoneId varchar(255) NOT NULL,
    CONSTRAINT cloudflare_pkey PRIMARY KEY (config_id),
    CONSTRAINT cloudflare_single_row CHECK (config_id = 1)
);
----
CREATE TABLE IF NOT EXISTS frontdesk.pihole (
    config_id int4 NOT NULL DEFAULT 1,
    url varchar(255) NOT NULL,
    password varchar(255) NOT NULL,
    CONSTRAINT pihole_pkey PRIMARY KEY (config_id),
    CONSTRAINT pihole_single_row CHECK (config_id = 1)
);
----
CREATE TABLE IF NOT EXISTS frontdesk.integrations_available (
    name varchar(50) NOT NULL,
    enabled bool NOT NULL,
    CONSTRAINT integrations_available_pkey PRIMARY KEY (name)
);
----
INSERT INTO frontdesk.integrations_available (name, enabled) 
VALUES ('cloudflare', false) ON CONFLICT (name) DO NOTHING;
----
INSERT INTO frontdesk.integrations_available (name, enabled) 
VALUES ('pihole', false) ON CONFLICT (name) DO NOTHING;
----
INSERT INTO frontdesk.integrations_available (name, enabled) 
VALUES ('transmission', false) ON CONFLICT (name) DO NOTHING;
----
CREATE TABLE IF NOT EXISTS frontdesk.widgets (
    id serial4 NOT NULL,
    name varchar(50) NOT NULL,
    enabled bool NOT NULL DEFAULT false,
    position int4 NOT NULL DEFAULT 999,
    selected bool NOT NULL DEFAULT false,
    CONSTRAINT widgets_pkey PRIMARY KEY (id)
);
----
CREATE OR REPLACE FUNCTION frontdesk.check_widgets_selected_limit()
RETURNS TRIGGER AS 
    $$
    BEGIN
        IF (SELECT count(*) FROM frontdesk.widgets where selected = true) >= 5 THEN
            RAISE EXCEPTION 'Cannot select more than 5 widgets to home';
        END IF;
        RETURN NEW;
    END;
    $$ 
    LANGUAGE plpgsql;
----
CREATE TRIGGER trigger_check_widgets_selected_limit
BEFORE INSERT OR UPDATE ON frontdesk.widgets
FOR EACH ROW EXECUTE FUNCTION frontdesk.check_widgets_selected_limit();