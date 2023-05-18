PLATFORMS := windows/amd64/.exe linux/amd64 darwin/amd64 illumos/amd64 windows/arm64/.exe android/arm64 linux/arm64 darwin/arm64 linux/arm freebsd/amd64

COUNT=3
GOARM=7
GOAMD64=v3
GOTAGS=-tags 'osusergo netgo'
LDFLAGS=-ldflags "-s -w -extldflags -static"

plat_temp = $(subst /, ,$@)
os = $(word 1, $(plat_temp))
arch = $(word 2, $(plat_temp))
ext = $(word 3, $(plat_temp))

.DEFAULT_GOAL := release

release: $(PLATFORMS)

compat: GOAMD64 = v1
compat: GOARM = 6
compat: ext = -compat$(word 3, $(plat_temp))
compat: release

$(PLATFORMS):
	CGO_ENABLED=0 GOOS=$(os) GOARCH=$(arch) GOARM=$(GOARM) GOAMD64=$(GOAMD64) go build $(GOTAGS) $(LDFLAGS) -o bin/quicns-$(os)-$(arch)$(ext) .
