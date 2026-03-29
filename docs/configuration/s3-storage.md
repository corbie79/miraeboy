# S3 스토리지 설정

miraeboy는 로컬 파일 시스템 대신 S3 호환 스토리지를 사용할 수 있습니다.
MinIO, AWS S3, Cloudflare R2, NCP Object Storage 등 S3 API를 지원하는 모든 서비스와 호환됩니다.

---

## 로컬 스토리지 vs S3 스토리지

| 항목 | 로컬 스토리지 | S3 스토리지 |
|------|--------------|------------|
| 설정 난이도 | 간단 | 보통 |
| 확장성 | 제한적 | 높음 |
| 고가용성 | 단일 서버 | 여러 노드 공유 가능 |
| 비용 | 서버 디스크 비용 | 스토리지 서비스 비용 |
| 백업 | 수동 | 스토리지 서비스 제공 |
| replica 노드 지원 | 복잡 | 권장 |

!!! tip "프로덕션 환경 권장"
    프로덕션 환경이나 replica 노드를 운영할 경우 S3 스토리지를 권장합니다.

---

## config.yaml 설정

```yaml
server:
  s3:
    endpoint: "http://minio.internal:9000"  # MinIO 엔드포인트 (AWS S3는 빈 값)
    bucket: "miraeboy"                       # 버킷 이름
    access_key_id: "your-access-key"
    secret_access_key: "your-secret-key"
    use_ssl: false                           # HTTPS 사용 여부
    region: ""                               # AWS 리전 (AWS S3 사용 시)
```

| 필드 | 설명 | 예시 |
|------|------|------|
| `endpoint` | S3 서비스 엔드포인트 URL. AWS S3는 빈 값 | `http://minio.internal:9000` |
| `bucket` | 패키지를 저장할 버킷 이름 | `miraeboy` |
| `access_key_id` | 액세스 키 ID | — |
| `secret_access_key` | 시크릿 액세스 키 | — |
| `use_ssl` | HTTPS 사용 여부 | `true` / `false` |
| `region` | AWS 리전 (AWS S3 전용) | `ap-northeast-2` |

---

## MinIO 설정

### MinIO 설치 (Docker)

```bash
docker run -d \
  --name minio \
  -p 9000:9000 \
  -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -v /data/minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"
```

### MinIO 버킷 생성

```bash
# mc (MinIO Client) 설치
curl -fsSL https://dl.min.io/client/mc/release/linux-amd64/mc -o /usr/local/bin/mc
chmod +x /usr/local/bin/mc

# MinIO 서버 등록
mc alias set local http://localhost:9000 minioadmin minioadmin

# 버킷 생성
mc mb local/miraeboy

# 버킷 확인
mc ls local
```

### miraeboy config.yaml (MinIO)

```yaml
server:
  s3:
    endpoint: "http://minio.internal:9000"
    bucket: "miraeboy"
    access_key_id: "minioadmin"
    secret_access_key: "minioadmin"
    use_ssl: false
    region: ""
```

### MinIO + TLS

MinIO에 TLS를 적용한 경우:

```yaml
server:
  s3:
    endpoint: "https://minio.internal:9000"
    bucket: "miraeboy"
    access_key_id: "minioadmin"
    secret_access_key: "minioadmin"
    use_ssl: true
    region: ""
```

---

## AWS S3 설정

### IAM 정책

miraeboy 전용 IAM 사용자와 최소 권한 정책을 생성합니다.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket",
        "s3:GetBucketLocation"
      ],
      "Resource": [
        "arn:aws:s3:::miraeboy-packages",
        "arn:aws:s3:::miraeboy-packages/*"
      ]
    }
  ]
}
```

### AWS S3 버킷 생성

```bash
# AWS CLI로 버킷 생성
aws s3api create-bucket \
  --bucket miraeboy-packages \
  --region ap-northeast-2 \
  --create-bucket-configuration LocationConstraint=ap-northeast-2

# 퍼블릭 액세스 차단
aws s3api put-public-access-block \
  --bucket miraeboy-packages \
  --public-access-block-configuration \
    BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
```

### miraeboy config.yaml (AWS S3)

```yaml
server:
  s3:
    endpoint: ""                    # AWS S3는 빈 값 (자동으로 AWS 엔드포인트 사용)
    bucket: "miraeboy-packages"
    access_key_id: "AKIAIOSFODNN7EXAMPLE"
    secret_access_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    use_ssl: true
    region: "ap-northeast-2"
```

### EC2 인스턴스 프로필 사용 (권장)

EC2에서 실행 중이라면 IAM 인스턴스 프로필을 사용하여 키를 하드코딩하지 않을 수 있습니다.

```yaml
server:
  s3:
    endpoint: ""
    bucket: "miraeboy-packages"
    access_key_id: ""     # 비워두면 인스턴스 프로필 사용
    secret_access_key: ""
    use_ssl: true
    region: "ap-northeast-2"
```

---

## Cloudflare R2 설정

Cloudflare R2는 S3 호환 API를 제공합니다.

```yaml
server:
  s3:
    endpoint: "https://<ACCOUNT_ID>.r2.cloudflarestorage.com"
    bucket: "miraeboy"
    access_key_id: "your-r2-access-key-id"
    secret_access_key: "your-r2-secret-access-key"
    use_ssl: true
    region: "auto"
```

R2 API 토큰 생성:
1. Cloudflare Dashboard → R2 → API Tokens
2. "Create API Token" 클릭
3. 권한: Object Read & Write
4. 생성된 Access Key ID, Secret Access Key 복사

---

## NCP Object Storage (네이버 클라우드) 설정

```yaml
server:
  s3:
    endpoint: "https://kr.object.ncloudstorage.com"
    bucket: "miraeboy"
    access_key_id: "your-ncp-access-key"
    secret_access_key: "your-ncp-secret-key"
    use_ssl: true
    region: "kr-standard"
```

---

## 버킷 구조

miraeboy가 S3 버킷에 저장하는 파일 구조:

```
miraeboy/
├── conan/
│   └── {repository}/
│       └── {namespace}/
│           └── {name}/
│               └── {version}/
│                   └── {channel}/
│                       └── {rrev}/
│                           ├── conanfile.py
│                           └── packages/
│                               └── {pkgid}/
│                                   └── {prev}/
│                                       └── conan_package.tgz
└── cargo/
    └── {repository}/
        └── {name}/
            └── {version}/
                └── {name}-{version}.crate
```

---

## 스토리지 마이그레이션

### 로컬 스토리지 → S3

1. MinIO에 기존 데이터를 업로드합니다.

```bash
# mc로 로컬 → MinIO 동기화
mc mirror /var/lib/miraeboy/data local/miraeboy
```

2. config.yaml을 S3 설정으로 변경합니다.

3. 서버를 재시작합니다.

```bash
sudo systemctl restart miraeboy
```

### S3 → 다른 S3

```bash
# mc로 버킷 간 동기화
mc mirror old-remote/miraeboy new-remote/miraeboy
```

---

## 버킷 수명 주기 정책 (선택)

오래된 임시 파일을 자동으로 삭제하는 정책을 설정할 수 있습니다.

```json
{
  "Rules": [
    {
      "ID": "expire-temp-files",
      "Status": "Enabled",
      "Filter": {
        "Prefix": "temp/"
      },
      "Expiration": {
        "Days": 7
      }
    }
  ]
}
```

```bash
aws s3api put-bucket-lifecycle-configuration \
  --bucket miraeboy-packages \
  --lifecycle-configuration file://lifecycle.json
```
