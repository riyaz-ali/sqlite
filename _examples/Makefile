# compile all extensions under _examples/

DIRS = $(patsubst %/,%,$(wildcard */))
EXT = so
ifeq ($(shell uname),Darwin)
	EXT = dylib
endif
LIBS = $(addsuffix .$(EXT),$(DIRS))

.PHONY: all
all: $(LIBS)

.PHONY: clean
clean:
	-rm -f $(LIBS)

%.$(EXT):
	go build -buildmode=c-shared -o $@ ./$*
