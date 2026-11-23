FROM docker.io/stalwartlabs/stalwart:v1.1.1

USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 \
    && rm -rf /var/lib/apt/lists/*
COPY start.py /opt/shiroyagi/start.py
COPY fixtures /opt/shiroyagi/fixtures

USER stalwart
ENTRYPOINT ["python3", "/opt/shiroyagi/start.py"]
CMD []
