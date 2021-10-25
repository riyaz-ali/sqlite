# compile all extensions under _examples/

.PHONY: all
all: compare.so series.so sum.so upper.so subtype.so

.PHONY: clean
clean:
	-rm -f compare.so series.so sum.so upper.so subtype.so

compare.so:
	 go build -buildmode=c-shared -o $@ ./compare

series.so:
	 go build -buildmode=c-shared -o $@ ./series

sum.so:
	 go build -buildmode=c-shared -o $@ ./sum

upper.so:
	go build -buildmode=c-shared -o $@ ./upper

csv.so:
	go build -buildmode=c-shared -o $@ ./csv

subtype.so:
	go build -buildmode=c-shared -o $@ ./subtype