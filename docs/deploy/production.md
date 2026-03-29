# 프로덕션 배포 가이드

miraeboy를 프로덕션 환경에 안전하게 배포하는 방법을 설명합니다.
nginx 리버스 프록시, TLS, Docker, 리소스 권장 사양을 다룹니다.

---

## 아키텍처 개요

```
인터넷/내부망
    │
    ▼
[nginx 리버스 프록시]
    │  HTTPS :443
    │  HTTP  :80 → 301 redirect
    ▼
[miraeboy 서버 :9300]
    │
    ├── [S3/MinIO 스토리지]
    ├── [Git 저장소 (선택)]
    └── [miraeboy-agent (빌드 서버)]
```

---

## 리소스 권장 사양

| 용도 | CPU | 메모리 | 디스크 |
|------|-----|--------|--------|
| 소규모 팀 (10명 이하) | 2코어 | 1GB | 50GB |
| 중간 규모 (50명 이하) | 4코어 | 4GB | 200GB |
| 대규모 (100명 이상) | 8코어+ | 8GB+ | S3 권장 |
| 에이전트 (빌드 서버) | 8코어 | 16GB | 100GB |

!!! tip "S3 스토리지 사용 권장"
    패키지 데이터는 S3 호환 스토리지에 저장하면 서버 디스크 부담이 없고
    replica 노드 운영도 쉬워집니다.

---

## nginx 리버스 프록시 설정

### nginx 설치

```bash
sudo apt install nginx
```

### nginx 설정 파일

```nginx
# /etc/nginx/sites-available/miraeboy
server {
    listen 80;
    server_name miraeboy.example.com;

    # HTTP → HTTPS 리디렉트
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name miraeboy.example.com;

    # TLS 인증서 (Let's Encrypt)
    ssl_certificate     /etc/letsencrypt/live/miraeboy.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/miraeboy.example.com/privkey.pem;

    # TLS 보안 설정
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 1d;

    # HSTS
    add_header Strict-Transport-Security "max-age=63072000" always;

    # 패키지 업로드를 위한 최대 파일 크기 설정 (필요에 따라 조정)
    client_max_body_size 500m;

    # 업로드 타임아웃 (큰 패키지 업로드를 위해)
    proxy_read_timeout 300s;
    proxy_connect_timeout 60s;
    proxy_send_timeout 300s;

    # 리버스 프록시 설정
    location / {
        proxy_pass http://127.0.0.1:9300;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket 지원 (필요 시)
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # 로그
    access_log /var/log/nginx/miraeboy.access.log;
    error_log  /var/log/nginx/miraeboy.error.log;
}
```

### nginx 활성화

```bash
sudo ln -s /etc/nginx/sites-available/miraeboy /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### Let's Encrypt TLS 인증서

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d miraeboy.example.com
sudo certbot renew --dry-run
```

---

## 자체 서명 인증서 (내부망)

공인 도메인 없이 내부망에서 사용할 경우:

```bash
# CA 키 및 인증서 생성
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
  -subj "/CN=Internal CA/O=MyCompany"

# 서버 키 및 CSR 생성
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr \
  -subj "/CN=miraeboy.internal/O=MyCompany"

# 서버 인증서 서명
cat > server.ext << 'EOF'
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
subjectAltName = @alt_names

[alt_names]
DNS.1 = miraeboy.internal
DNS.2 = miraeboy.company.local
IP.1 = 192.168.1.100
EOF

openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt -extfile server.ext

# 인증서 배포
sudo cp server.crt /etc/nginx/certs/miraeboy.crt
sudo cp server.key /etc/nginx/certs/miraeboy.key
sudo chmod 600 /etc/nginx/certs/miraeboy.key
```

클라이언트(개발자 PC)에 CA 인증서 배포:

```bash
# Ubuntu/Debian
sudo cp ca.crt /usr/local/share/ca-certificates/mycompany-ca.crt
sudo update-ca-certificates

# macOS
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain ca.crt

# Windows (PowerShell, 관리자)
Import-Certificate -FilePath ca.crt -CertStoreLocation Cert:\LocalMachine\Root
```

---

## Docker 배포

### Dockerfile

```dockerfile
# Dockerfile.server
FROM debian:12-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY miraeboy /usr/local/bin/miraeboy
RUN chmod +x /usr/local/bin/miraeboy

RUN useradd --system --no-create-home --shell /bin/false miraeboy
RUN mkdir -p /var/lib/miraeboy/data /etc/miraeboy && \
    chown -R miraeboy:miraeboy /var/lib/miraeboy /etc/miraeboy

USER miraeboy
EXPOSE 9300

CMD ["/usr/local/bin/miraeboy", "--config", "/etc/miraeboy/config.yaml"]
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  miraeboy:
    image: miraeboy:latest
    container_name: miraeboy
    restart: unless-stopped
    ports:
      - "9300:9300"
    volumes:
      - ./config.yaml:/etc/miraeboy/config.yaml:ro
      - miraeboy-data:/var/lib/miraeboy/data
      - miraeboy-git:/var/lib/miraeboy/git-workspace
    environment:
      - JWT_SECRET=${JWT_SECRET}
      - AGENT_KEY=${AGENT_KEY}
    networks:
      - miraeboy-net

  minio:
    image: quay.io/minio/minio:latest
    container_name: minio
    restart: unless-stopped
    command: server /data --console-address ":9001"
    ports:
      - "9001:9001"   # MinIO Console (내부 접근용)
    volumes:
      - minio-data:/data
    environment:
      - MINIO_ROOT_USER=${MINIO_ROOT_USER}
      - MINIO_ROOT_PASSWORD=${MINIO_ROOT_PASSWORD}
    networks:
      - miraeboy-net

  nginx:
    image: nginx:alpine
    container_name: nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/conf.d/miraeboy.conf:ro
      - ./certs:/etc/nginx/certs:ro
    depends_on:
      - miraeboy
    networks:
      - miraeboy-net

volumes:
  miraeboy-data:
  miraeboy-git:
  minio-data:

networks:
  miraeboy-net:
    driver: bridge
```

### .env 파일

```bash
# .env
JWT_SECRET=your-very-long-random-jwt-secret-key
AGENT_KEY=your-agent-key
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin-password-change-me
```

### 실행

```bash
docker compose up -d
docker compose logs -f miraeboy
```

---

## systemd 서비스 (권장)

Docker 없이 직접 서비스로 운영하는 방법입니다.

### 서비스 계정 생성

```bash
sudo useradd --system --no-create-home --shell /bin/false miraeboy
sudo mkdir -p /var/lib/miraeboy/{data,artifacts,git-workspace}
sudo mkdir -p /etc/miraeboy
sudo chown -R miraeboy:miraeboy /var/lib/miraeboy /etc/miraeboy
```

### 설정 파일 배포

```bash
sudo cp config.yaml /etc/miraeboy/config.yaml
sudo chmod 640 /etc/miraeboy/config.yaml
sudo chown root:miraeboy /etc/miraeboy/config.yaml
```

### 환경 변수 파일

```bash
sudo tee /etc/miraeboy/env << 'EOF'
JWT_SECRET=your-secret-key
AGENT_KEY=your-agent-key
EOF
sudo chmod 600 /etc/miraeboy/env
sudo chown root:miraeboy /etc/miraeboy/env
```

### systemd 유닛

```bash
sudo tee /etc/systemd/system/miraeboy.service << 'EOF'
[Unit]
Description=miraeboy Package Registry Server
Documentation=https://corbie79.github.io/miraeboy
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=miraeboy
Group=miraeboy
EnvironmentFile=/etc/miraeboy/env
ExecStart=/usr/local/bin/miraeboy --config /etc/miraeboy/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s
TimeoutStopSec=30s

# 보안 강화
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/miraeboy

# 리소스 제한
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now miraeboy
```

---

## 모니터링

### 헬스 체크

```bash
# 기본 헬스 체크
curl http://localhost:9300/ping

# systemd에서 헬스 체크 타이머 설정
sudo tee /etc/systemd/system/miraeboy-healthcheck.service << 'EOF'
[Unit]
Description=miraeboy Health Check
After=miraeboy.service

[Service]
Type=oneshot
ExecStart=/bin/bash -c 'curl -sf http://localhost:9300/ping || systemctl restart miraeboy'
EOF

sudo tee /etc/systemd/system/miraeboy-healthcheck.timer << 'EOF'
[Unit]
Description=miraeboy Health Check Timer

[Timer]
OnBootSec=60s
OnUnitActiveSec=60s

[Install]
WantedBy=timers.target
EOF

sudo systemctl enable --now miraeboy-healthcheck.timer
```

### 로그 관리

```bash
# 실시간 로그
sudo journalctl -u miraeboy -f

# 최근 100줄
sudo journalctl -u miraeboy -n 100

# 특정 기간 로그
sudo journalctl -u miraeboy --since "2024-03-15" --until "2024-03-16"

# 로그 보존 기간 설정 (30일)
sudo tee /etc/systemd/journald.conf.d/miraeboy.conf << 'EOF'
[Journal]
MaxRetentionSec=30day
EOF
```

---

## 백업

### 로컬 스토리지 백업

```bash
#!/bin/bash
# /usr/local/bin/miraeboy-backup.sh

BACKUP_DIR="/backup/miraeboy/$(date +%Y%m%d)"
STORAGE_PATH="/var/lib/miraeboy/data"

mkdir -p "$BACKUP_DIR"

# 데이터 디렉터리 백업
tar -czf "$BACKUP_DIR/data.tar.gz" "$STORAGE_PATH"

# 설정 파일 백업
cp /etc/miraeboy/config.yaml "$BACKUP_DIR/"

# 30일 이상 된 백업 삭제
find /backup/miraeboy -type d -mtime +30 -exec rm -rf {} +

echo "백업 완료: $BACKUP_DIR"
```

```bash
# 일일 백업 cron 등록
echo "0 2 * * * root /usr/local/bin/miraeboy-backup.sh" | sudo tee /etc/cron.d/miraeboy-backup
```

### S3 스토리지 백업

S3를 사용하면 스토리지 서비스의 버전 관리와 복제 기능을 활용하세요.

```bash
# AWS S3 Cross-Region Replication 활성화 (AWS CLI)
aws s3api put-bucket-replication \
  --bucket miraeboy-packages \
  --replication-configuration file://replication.json
```

---

## 업그레이드

```bash
# 1. 새 버전 다운로드
curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install.sh | sh

# 2. 서비스 재시작
sudo systemctl restart miraeboy

# 3. 상태 확인
sudo systemctl status miraeboy
curl http://localhost:9300/ping

# 4. 로그 확인
sudo journalctl -u miraeboy -n 50
```

!!! note "무중단 업그레이드"
    miraeboy는 현재 단일 프로세스로 동작하므로 재시작 시 짧은 다운타임이 발생합니다.
    다운타임 없는 업그레이드가 필요하다면 로드 밸런서와 replica 노드를 함께 운영하세요.

---

## 보안 체크리스트

- [ ] `jwt_secret`을 강력한 랜덤 문자열로 설정
- [ ] `agent_key`를 강력한 랜덤 문자열로 설정
- [ ] TLS 인증서 적용
- [ ] 방화벽에서 9300 포트를 직접 외부 노출 차단 (nginx를 통해서만 접근)
- [ ] 민감한 환경 변수를 `/etc/miraeboy/env`에서 600 권한으로 관리
- [ ] 정기적인 백업 설정
- [ ] 로그 모니터링 설정
- [ ] `anonymous_access`가 불필요하게 `write`로 설정된 리포지토리 없는지 확인
- [ ] 관리자 계정 비밀번호를 강력한 값으로 변경
- [ ] 불필요한 사용자 계정 제거
