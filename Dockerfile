FROM alpine:latest
ADD main /
CMD ["/main"]
EXPOSE 80

