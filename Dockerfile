FROM gliderlabs/alpine:3.4

ADD bin/lambda-resource-linux-amd64 /opt/resource/out

RUN cd /opt/resource && ln -s out check && ln -s out in
