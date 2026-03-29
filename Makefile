# miraeboy cross-platform build
# 단일 머신에서 모든 플랫폼 빌드 (Go 크로스 컴파일)
#
# 사용법:
#   make                      # 전체 빌드 (모든 플랫폼)
#   make linux                # linux amd64 + arm64
#   make darwin               # macOS Intel + Apple Silicon
#   make windows              # windows amd64
#   make release VERSION=v1.0.0
#   make clean

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   = -s -w -X main.Version=$(VERSION)
OUTDIR    = dist
BINARY    = miraeboy
AGENT     = miraeboy-agent

.PHONY: all web linux darwin windows agent release clean help

all: web linux darwin windows agent

help:
	@echo "make [target] [VERSION=v1.0.0]"
	@echo ""
	@echo "  all      모든 플랫폼 빌드 + agent (기본값)"
	@echo "  linux    linux/amd64, linux/arm64"
	@echo "  darwin   darwin/amd64 (Intel), darwin/arm64 (Apple Silicon)"
	@echo "  windows  windows/amd64"
	@echo "  agent    miraeboy-agent 전 플랫폼 빌드"
	@echo "  release  빌드 + 아카이브 생성 (.tar.gz / .zip)"
	@echo "  clean    dist/ 삭제"

web:
	@echo "==> Frontend build"
	@cd web && npm ci --silent && npm run build --silent

$(OUTDIR):
	@mkdir -p $(OUTDIR)

linux: web $(OUTDIR)
	@echo "==> linux/amd64"
	@GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-linux-amd64 .
	@echo "==> linux/arm64"
	@GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-linux-arm64 .

darwin: web $(OUTDIR)
	@echo "==> darwin/amd64"
	@GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-darwin-amd64 .
	@echo "==> darwin/arm64"
	@GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-darwin-arm64 .

windows: web $(OUTDIR)
	@echo "==> windows/amd64"
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-windows-amd64.exe .

agent: $(OUTDIR)
	@echo "==> miraeboy-agent linux/amd64"
	@GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(AGENT)-linux-amd64   ./cmd/build-agent
	@echo "==> miraeboy-agent linux/arm64"
	@GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(AGENT)-linux-arm64   ./cmd/build-agent
	@echo "==> miraeboy-agent darwin/amd64"
	@GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(AGENT)-darwin-amd64  ./cmd/build-agent
	@echo "==> miraeboy-agent darwin/arm64"
	@GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(AGENT)-darwin-arm64  ./cmd/build-agent
	@echo "==> miraeboy-agent windows/amd64"
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(OUTDIR)/$(AGENT)-windows-amd64.exe ./cmd/build-agent

release: all
	@echo "==> Creating archives..."
	@cd $(OUTDIR) && \
	  for f in $(BINARY)-linux-amd64 $(BINARY)-linux-arm64 \
	           $(BINARY)-darwin-amd64 $(BINARY)-darwin-arm64 \
	           $(AGENT)-linux-amd64  $(AGENT)-linux-arm64 \
	           $(AGENT)-darwin-amd64 $(AGENT)-darwin-arm64; do \
	    tar czf $$f-$(VERSION).tar.gz $$f && echo "  $$f-$(VERSION).tar.gz"; \
	  done
	@cd $(OUTDIR) && \
	  zip -q $(BINARY)-windows-amd64-$(VERSION).zip $(BINARY)-windows-amd64.exe && \
	  echo "  $(BINARY)-windows-amd64-$(VERSION).zip" && \
	  zip -q $(AGENT)-windows-amd64-$(VERSION).zip   $(AGENT)-windows-amd64.exe  && \
	  echo "  $(AGENT)-windows-amd64-$(VERSION).zip"
	@echo "==> Done. Artifacts in $(OUTDIR)/"

clean:
	@rm -rf $(OUTDIR)
	@echo "cleaned."
