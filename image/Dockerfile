FROM python:3
RUN mkdir /autoscale/
COPY ./script/requirements.txt /tmp
RUN pip3 install -r /tmp/requirements.txt
COPY ./script/* /autoscale/
WORKDIR /autoscale
ENV PYTHONUNBUFFERED=1
CMD ["python3", "-u", "./autoscale.py"]