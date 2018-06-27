FROM ubuntu:16.04
RUN apt update && apt -y install curl && curl https://raw.githubusercontent.com/kubernetes/helm/master/scripts/get | bash
RUN helm init --client-only
ADD main /
CMD ["/main"]
EXPOSE 80

