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
CREATE TABLE IF NOT EXISTS frontdesk.transmission (
    config_id int4 NOT NULL DEFAULT 1,
    url varchar(255) NOT NULL,
    username varchar(255) NOT NULL,
    password varchar(255) NOT NULL,
    port int4 NULL,
    protocol varchar(10) NOT NULL DEFAULT 'http',
    CONSTRAINT transmission_pkey PRIMARY KEY (config_id),
    CONSTRAINT transmission_single_row CHECK (config_id = 1)
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
    CONSTRAINT widgets_pkey PRIMARY KEY (id),
    CONSTRAINT widgets_name_unique UNIQUE (name)
);
----
CREATE OR REPLACE FUNCTION frontdesk.check_widgets_selected_limit()
RETURNS TRIGGER AS 
    $$
    BEGIN
        IF NEW.selected = true
            AND (TG_OP = 'INSERT' OR OLD.selected = false)
            AND (SELECT count(*) FROM frontdesk.widgets where selected = true and id <> NEW.id) >= 5 THEN
            RAISE EXCEPTION 'Cannot select more than 5 widgets to home';
        END IF;
        RETURN NEW;
    END;
    $$ 
    LANGUAGE plpgsql;
----
DROP TRIGGER IF EXISTS trigger_check_widgets_selected_limit ON frontdesk.widgets;
CREATE TRIGGER trigger_check_widgets_selected_limit
BEFORE INSERT OR UPDATE ON frontdesk.widgets
FOR EACH ROW EXECUTE FUNCTION frontdesk.check_widgets_selected_limit();
----
CREATE TABLE IF NOT EXISTS frontdesk.image_update_checks (
    image_ref varchar(512) NOT NULL,
    local_digest varchar(128) NULL,
    remote_digest varchar(128) NULL,
    status varchar(20) NOT NULL,
    error text NULL,
    checked_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT image_update_checks_pkey PRIMARY KEY (image_ref)
);
----
CREATE TABLE IF NOT EXISTS frontdesk.cloudflare_routes (
    hostname varchar(255) NOT NULL,
    app_dir varchar(255) NULL,
    service varchar(255) NOT NULL,
    dns_record_id varchar(64) NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT cloudflare_routes_pkey PRIMARY KEY (hostname)
);
----
CREATE TABLE IF NOT EXISTS frontdesk.health_checks (
    id bigserial NOT NULL,
    app_key varchar(255) NOT NULL,
    app_name varchar(255) NULL,
    container_status varchar(32) NULL,
    http_status int4 NULL,
    latency_ms int4 NULL,
    up bool NOT NULL,
    error text NULL,
    checked_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT health_checks_pkey PRIMARY KEY (id)
);
----
CREATE INDEX IF NOT EXISTS health_checks_app_time_idx ON frontdesk.health_checks (app_key, checked_at DESC);
----
CREATE INDEX IF NOT EXISTS health_checks_time_idx ON frontdesk.health_checks (checked_at);
----
CREATE TABLE IF NOT EXISTS frontdesk.monitor_state (
    app_key varchar(255) NOT NULL,
    up bool NOT NULL,
    consecutive_failures int4 NOT NULL DEFAULT 0,
    changed_at timestamptz NOT NULL DEFAULT now(),
    last_checked_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT monitor_state_pkey PRIMARY KEY (app_key)
);
----
CREATE TABLE IF NOT EXISTS frontdesk.host_metrics (
    id bigserial NOT NULL,
    cpu_percent real NOT NULL,
    mem_percent real NOT NULL,
    disk_percent real NULL,
    load1 real NULL,
    sampled_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT host_metrics_pkey PRIMARY KEY (id)
);
----
CREATE INDEX IF NOT EXISTS host_metrics_time_idx ON frontdesk.host_metrics (sampled_at DESC);
----
CREATE TABLE IF NOT EXISTS frontdesk.notification_channels (
    id serial4 NOT NULL,
    name varchar(100) NOT NULL,
    type varchar(20) NOT NULL,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    events text[] NOT NULL DEFAULT ARRAY['down','up'],
    enabled bool NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notification_channels_pkey PRIMARY KEY (id),
    CONSTRAINT notification_channels_type_chk CHECK (type IN ('ntfy','discord','telegram','webhook'))
);
----
CREATE TABLE IF NOT EXISTS frontdesk.nginx_proxy_manager (
    config_id int4 NOT NULL DEFAULT 1,
    url varchar(255) NOT NULL,
    email varchar(255) NOT NULL,
    password varchar(255) NOT NULL,
    forward_host varchar(255) NOT NULL,
    default_certificate_id int4 NOT NULL DEFAULT 0,
    CONSTRAINT nginx_proxy_manager_pkey PRIMARY KEY (config_id),
    CONSTRAINT nginx_proxy_manager_single_row CHECK (config_id = 1)
);
----
INSERT INTO frontdesk.integrations_available (name, enabled) 
VALUES ('nginxproxymanager', false) ON CONFLICT (name) DO NOTHING;
----
CREATE TABLE IF NOT EXISTS frontdesk.npm_routes (
    proxy_host_id int4 NOT NULL,
    hostname varchar(255) NOT NULL,
    app_dir varchar(255) NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT npm_routes_pkey PRIMARY KEY (proxy_host_id)
);
----
ALTER TABLE frontdesk.users ADD COLUMN IF NOT EXISTS is_admin bool NOT NULL DEFAULT false;
----
UPDATE frontdesk.users SET is_admin = true
WHERE id = (SELECT min(id) FROM frontdesk.users)
AND NOT EXISTS (SELECT 1 FROM frontdesk.users WHERE is_admin);
