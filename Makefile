#            _ _ _                       _
#  ___  __ _| (_| |_ ___        _____  _| |_
# / __|/ _` | | | __/ _ \_____ / _ \ \/ | __|
# \__ | (_| | | | ||  __|_____|  __/>  <| |_
# |___/\__, |_|_|\__\___|      \___/_/\_\\__|
#         |_|
.PHONY: vet test

# pass these flags to linker to suppress missing symbol errors in intermediate artifacts
export CGO_LDFLAGS = -Wl,--unresolved-symbols=ignore-in-object-files
ifeq ($(shell uname -s),Darwin)
	export CGO_LDFLAGS = -Wl,-undefined,dynamic_lookup
endif

# go build tags used by test, vet and more
TAGS = "libsqlite3"

# ========================================
# target for common golang tasks

vet:
	@go vet -v -tags=$(TAGS)

test:
	@go test -v -tags=$(TAGS)
