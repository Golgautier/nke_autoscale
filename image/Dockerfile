# Building image 

FROM golang:1.19.3-bullseye as build-stage

RUN mkdir -p /autoscale-building/ntnx_api_call
WORKDIR /autoscale-building
COPY script/go.mod script/go.sum ./
RUN go mod download
COPY script/*.go .
COPY script/ntnx_api_call/* ntnx_api_call/
RUN CGO_ENABLED=0 GOOS=linux go build -o ./nke-autoscale
CMD ["./nke-autoscale"]


FROM scratch

# Copy binary
COPY --from=build-stage /autoscale-building/nke-autoscale /nke-autoscale

ENTRYPOINT ["/nke-autoscale"]