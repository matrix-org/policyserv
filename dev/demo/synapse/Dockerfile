FROM matrixdotorg/synapse:latest

ARG UID

# for munging the config it spits out
RUN apt-get update && apt install wget patch -y
RUN wget -O yq "https://github.com/mikefarah/yq/releases/download/v4.35.1/yq_linux_amd64" && chmod u+x yq

COPY ca/certs/ca.crt /usr/local/share/ca-certificates/myca.crt
RUN update-ca-certificates

ADD synapse/compose-start.sh /compose-start.sh
RUN chmod 770 /compose-start.sh
ENTRYPOINT [ "/compose-start.sh" ]
