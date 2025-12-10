#!/bin/bash -eu

echo "This script requires curl and jq to work"

echo "Checking HS1 is reachable..."
curl -XGET -fSsk 'https://localhost:4640/_matrix/client/versions'
echo ""
echo ""


echo "Checking HS2 is reachable..."
curl -XGET -fSsk 'https://localhost:4641/_matrix/client/versions'
echo ""
echo ""

echo "Checking policyserv is reachable..."
curl -XGET -fSs 'http://localhost:4642/ready'
echo ""
echo ""

# Don't use alice/bob to ensure we can re-run this script without it failing.
HS1_USER=$(head -c16 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c10)
HS2_USER=$(head -c16 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c10)
echo "Creating user '$HS1_USER' on HS1 and '$HS2_USER' on HS2..."
HS1_TOKEN=$(curl -XPOST -fSsk -d "{\"auth\":{\"type\":\"m.login.dummy\"},\"username\":\"$HS1_USER\",\"password\":\"$HS1_USER\"}" "https://localhost:4640/_matrix/client/v3/register" | jq -r .access_token)
HS2_TOKEN=$(curl -XPOST -fSsk -d "{\"auth\":{\"type\":\"m.login.dummy\"},\"username\":\"$HS2_USER\",\"password\":\"$HS2_USER\"}" "https://localhost:4641/_matrix/client/v3/register" | jq -r .access_token)
echo "$HS1_USER on HS1 (https://localhost:4640) access token: $HS1_TOKEN"
echo "$HS2_USER on HS2 (https://localhost:4641) access token: $HS2_TOKEN"

echo "Creating a room between HS1 <---> HS2"
ROOM_ID=$(curl -XPOST -fSsk -H "Authorization: Bearer $HS1_TOKEN" -d '{"preset":"public_chat"}' "https://localhost:4640/_matrix/client/v3/createRoom" | jq -r .room_id)
echo "Created room on HS1: $ROOM_ID"
echo "Joining room from HS2"
curl -XPOST -fSsk -H "Authorization: Bearer $HS2_TOKEN" -d '{}' "https://localhost:4641/_matrix/client/v3/join/$ROOM_ID?via=hs1"

# Send a policy server state event so we start checking events
echo ""
echo "Sending policy state event into room..."
POLICYSERV_KEY=$(curl -XGET -fSsk "https://localhost:4643/_matrix/key/v2/server" | jq -r '.verify_keys[] | .key')
curl -XPUT -fSsk -H "Authorization: Bearer $HS1_TOKEN" -d "{\"via\":\"policyserv\",\"public_key\":\"$POLICYSERV_KEY\"}" "https://localhost:4640/_matrix/client/v3/rooms/$ROOM_ID/state/org.matrix.msc4284.policy/"

echo ""
echo "Joining from policyserv..."
curl -XPOST -fSsk -H "Authorization: Bearer dontuseinproduction" -d "{\"via\":\"hs1\",\"room_ids\": [\"$ROOM_ID\"]}" "https://localhost:4643/api/v1/join_rooms"

echo ""
echo "Sending non-spam"
curl -XPUT -fSsk -H "Authorization: Bearer $HS1_TOKEN" -d '{"msgtype":"m.text","body":"this is ok"}' "https://localhost:4640/_matrix/client/v3/rooms/$ROOM_ID/send/m.room.message/3"

sleep 1
echo ""
echo "Sending spam"
curl -XPUT -fSsk -H "Authorization: Bearer $HS1_TOKEN" -d '{"msgtype":"m.text","body":"this is spam spam spam"}' "https://localhost:4640/_matrix/client/v3/rooms/$ROOM_ID/send/m.room.message/244"
