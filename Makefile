dc_cmd=docker-compose -p atx
atx_server_addr=192.168.147.230:8000
# --serial $SERIAL

up:
	$(dc_cmd) up -d --build

down:
	$(dc_cmd) down

shell:
	docker exec -it atxserver bash

ps:
	$(dc_cmd) ps

log:
	$(dc_cmd) logs -f atxserver

init:
	python -m uiautomator2 init --server $(atx_server_addr)