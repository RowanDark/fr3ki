.PHONY: build clean

BIN := bin/fr3ki
CMD := ./cmd/fr3ki

build:
	go build -o $(BIN) $(CMD)

clean:
	rm -f $(BIN)
