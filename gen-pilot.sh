go get -d -v ./pilot/... ; go build -o ~/tmp ./pilot/...

# to run the pilot:
# export ENABLE_CA_SERVER=false ; ~/tmp/pilot-discovery discovery --meshConfig ~/workplace/istio_configs/mesh-config.yaml --registries "" --vklog=9 --log_output_level=all:debug
