.PHONY: all toqart webui run-webui clean

all: toqart webui

toqart:
	cargo build --release

webui:
	cd webui && go build -ldflags="-X main.buildTime=$$(date +%Y%m%d_%H%M%S)" -o toqart-webui .

run-webui: webui
	cd webui && ./toqart-webui

clean:
	cargo clean
	rm -f webui/toqart-webui
