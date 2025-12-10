This directory contains a makefile for regenerating the CA and certs for all the demo servers.
You shouldn't need to run this for 1 year, as we check in the certs. All servers in the
`docker-compose.yml` file for the demo mount the `certs` directory.

```bash
# Create a CA cert in /certs/ca.{key|crt}
make ca

# Create a self-signed cert derived from the CA cert for the given hostname
# Produces $HOST.crt $HOST.csr $HOST.ext $HOST.key
make cert HOST=hs1
make cert HOST=hs2
make cert HOST=policyserv
```