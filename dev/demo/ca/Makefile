CERTS_DIR=certs
DAYS=365
CA_KEY=$(CERTS_DIR)/ca.key
CA_CERT=$(CERTS_DIR)/ca.crt
CA_SERIAL=$(CERTS_DIR)/ca.srl

HOST?=localhost

.PHONY: all ca cert clean trust

all: ca cert

# Create certs directory
$(CERTS_DIR):
	mkdir -p $(CERTS_DIR)

# Generate the root CA (key + cert)
ca: $(CERTS_DIR)
	openssl genrsa -out $(CA_KEY) 4096
	openssl req -x509 -new -nodes -key $(CA_KEY) -sha256 -days 3650 \
		-out $(CA_CERT) -subj "/CN=MyLocalCA"

# Generate a server cert signed by the CA
cert: $(CERTS_DIR)
	openssl genrsa -out $(CERTS_DIR)/$(HOST).key 2048
	openssl req -new -key $(CERTS_DIR)/$(HOST).key \
		-out $(CERTS_DIR)/$(HOST).csr -subj "/CN=$(HOST)"
	@echo "authorityKeyIdentifier=keyid,issuer" > $(CERTS_DIR)/$(HOST).ext
	@echo "basicConstraints=CA:FALSE" >> $(CERTS_DIR)/$(HOST).ext
	@echo "keyUsage = digitalSignature, keyEncipherment" >> $(CERTS_DIR)/$(HOST).ext
	@echo "extendedKeyUsage = serverAuth" >> $(CERTS_DIR)/$(HOST).ext
	@echo "subjectAltName = @alt_names" >> $(CERTS_DIR)/$(HOST).ext
	@echo "[alt_names]" >> $(CERTS_DIR)/$(HOST).ext
	@echo "DNS.1 = $(HOST)" >> $(CERTS_DIR)/$(HOST).ext
	@echo "DNS.2 = localhost" >> $(CERTS_DIR)/$(HOST).ext
	openssl x509 -req -in $(CERTS_DIR)/$(HOST).csr \
		-CA $(CA_CERT) -CAkey $(CA_KEY) -CAcreateserial \
		-out $(CERTS_DIR)/$(HOST).crt -days $(DAYS) -sha256 \
		-extfile $(CERTS_DIR)/$(HOST).ext