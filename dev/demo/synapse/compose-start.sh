#!/bin/bash -e

# The following is stolen from Chaos which needs generous rate limits
export SYNAPSE_REPORT_STATS=no

if [ -f /data/homeserver.yaml ]; then
    echo "homeserver.yaml already detected, not regenerating config"
    /start.py run
    exit 0
fi

echo " ====== Generating config  ====== "
/start.py generate

# Critical to enable DNS lookups locally as by default Synapse blocks private IPs
/yq -i '.ip_range_blacklist = []' /data/homeserver.yaml

# Allow open registration
/yq -i '.enable_registration = true' /data/homeserver.yaml
/yq -i '.enable_registration_without_verification = true' /data/homeserver.yaml

# Don't hit any non-test servers
/yq -i '.trusted_key_servers = []' /data/homeserver.yaml

# Provide TLS certs for listening on :443
/yq -i ".tls_certificate_path = \"/certs/$SYNAPSE_SERVER_NAME.crt\"" /data/homeserver.yaml
/yq -i ".tls_private_key_path = \"/certs/$SYNAPSE_SERVER_NAME.key\"" /data/homeserver.yaml

# Listen on :443 and serve up a .well-known response pointing to :443
/yq -i '.listeners = [{"port":443,"tls":true,"type":"http","resources":[{"names":["client","federation"]}]}]' /data/homeserver.yaml
/yq -i ".serve_server_wellknown = true" /data/homeserver.yaml

# Reduce backoff timers on federation for testing
/yq -i '.federation.destination_max_retry_interval = "10s"' /data/homeserver.yaml
/yq -i '.federation.destination_min_retry_interval = "1s"' /data/homeserver.yaml
/yq -i '.federation.max_short_retry_delay = "10s"' /data/homeserver.yaml
/yq -i '.federation.max_long_retry_delay = "10s"' /data/homeserver.yaml

# if postgres env vars are provided, use them instead of sqlite
if [[ -z $POSTGRES_DB || -z $POSTGRES_HOST || -z $POSTGRES_USER || -z $POSTGRES_PASSWORD ]]; then
  echo 'running in sqlite mode'
else
  echo 'running in postgres mode'
  /yq -i ".database = {\"name\":\"psycopg2\", \"args\":{\"user\":\"$POSTGRES_USER\",\"password\":\"$POSTGRES_PASSWORD\",\"dbname\":\"$POSTGRES_DB\",\"host\":\"$POSTGRES_HOST\",\"cp_min\":5,\"cp_max\":10}}" /data/homeserver.yaml
fi

# set rate limiting stuff
/yq -i '.rc_message = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_registration = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_login.address = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_login.account = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_login.failed_attempts = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_login.address = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_admin_redaction = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_joins.local = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_joins.remote = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_invites.per_room = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_invites.per_user = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_invites.per_issuer = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_3pid_validation = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_joins_per_room = {"per_second":1000,"burst_count":1000}' /data/homeserver.yaml
/yq -i '.rc_federation = {"sleep_delay":1}' /data/homeserver.yaml

echo " ====== Starting server with:  ====== "
cat /data/homeserver.yaml
echo  " ====== STARTING  ====== " 
/start.py run
