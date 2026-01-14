# YouTube to RTSP Proxy

YouTube 라이브 스트림을 RTSP 프로토콜로 변환하여 제공하는 Go 기반 CLI 도구입니다.

## 특징

- YouTube 영상/라이브 스트림을 RTSP로 프록시
- 라이브 스트림 URL 자동 갱신 (URL 만료 대응)
- 스트림 상태 모니터링 및 자동 재연결
- 다중 스트림 동시 지원
- 설정 파일 및 환경 변수 지원
- sudo 권한 없이 사용자 디렉토리 설치 가능
- 의존성 자동 설치 지원

## 아키텍처

```
YouTube Live Stream
        ↓
    yt-dlp (URL 추출)
        ↓
    FFmpeg (스트림 변환)
        ↓
    MediaMTX (RTSP 서버)
        ↓
    ┌─────────────┬─────────────┐
    ↓             ↓             ↓
  VLC        ffplay         기타 RTSP 클라이언트
```

## 요구사항

- Go 1.21+ (빌드 시)
- Linux / macOS (지원 플랫폼)

### 의존성 (자동 설치 가능)

| 도구 | 용도 | 자동 설치 |
|------|------|----------|
| ffmpeg | 스트림 변환 | 패키지 매니저 |
| yt-dlp | YouTube URL 추출 | pip 또는 바이너리 |
| MediaMTX | RTSP 서버 | GitHub 릴리즈 |

## 설치

### 빠른 설치 (권장)

```bash
# 저장소 클론 및 빌드
git clone https://github.com/zerodice0/youtube-rtsp-proxy.git
cd youtube-rtsp-proxy
make build

# 옵션 1: 시스템 전체 설치 (sudo 필요)
sudo ./scripts/install.sh --install-deps

# 옵션 2: 사용자 디렉토리 설치 (sudo 불필요)
./scripts/install.sh --user --install-deps
source ~/.bashrc  # PATH 적용
```

### 설치 옵션

```bash
./scripts/install.sh [옵션]
```

| 옵션 | 설명 |
|------|------|
| `--user` | `~/.local/bin`에 설치 (sudo 불필요) |
| `--prefix <path>` | 지정 경로에 설치 (예: `--prefix ~/tools`) |
| `--install-deps` | 의존성(ffmpeg, yt-dlp, mediamtx) 자동 설치 |
| `--dry-run` | 실제 설치 없이 동작 확인 |
| `--help` | 도움말 표시 |

### 설치 예시

```bash
# 사용자 디렉토리에 의존성과 함께 설치
./scripts/install.sh --user --install-deps

# 커스텀 경로에 설치
./scripts/install.sh --prefix ~/mytools --install-deps

# 설치 전 미리보기
./scripts/install.sh --user --install-deps --dry-run
```

### 설치 경로

| 모드 | 바이너리 | 설정 파일 | 데이터 |
|------|----------|-----------|--------|
| 시스템 (sudo) | `/usr/local/bin` | `/etc/youtube-rtsp-proxy` | `/var/lib/youtube-rtsp-proxy` |
| 사용자 (--user) | `~/.local/bin` | `~/.config/youtube-rtsp-proxy` | `~/.local/share/youtube-rtsp-proxy` |
| 커스텀 (--prefix) | `<prefix>/bin` | `<prefix>/etc/youtube-rtsp-proxy` | `<prefix>/share/youtube-rtsp-proxy` |

### 수동 빌드

```bash
# 현재 플랫폼
make build

# Linux (amd64, arm64)
make build-linux

# macOS (amd64, arm64)
make build-darwin

# 모든 플랫폼
make build-all
```

### 의존성 수동 설치

의존성을 직접 설치하려면:

```bash
# ffmpeg
sudo apt install ffmpeg      # Debian/Ubuntu
sudo dnf install ffmpeg      # Fedora
brew install ffmpeg          # macOS

# yt-dlp
pip install yt-dlp

# MediaMTX
# https://github.com/bluenviron/mediamtx/releases 에서 다운로드
```

## 사용법

### 빠른 시작

```bash
# 1. 서버 시작
youtube-rtsp-proxy server start

# 2. YouTube 스트림 프록시 시작
youtube-rtsp-proxy start "https://www.youtube.com/watch?v=jfKfPfyJRdk" --name lofi

# 3. 스트림 재생
ffplay rtsp://localhost:8554/lofi
# 또는
vlc rtsp://localhost:8554/lofi
```

### 스트림 관리

```bash
# 스트림 시작
youtube-rtsp-proxy start <youtube-url> --name <이름>

# 특정 포트로 시작
youtube-rtsp-proxy start <youtube-url> --name news --port 8555

# 스트림 목록 확인
youtube-rtsp-proxy list

# 스트림 상태 상세 확인
youtube-rtsp-proxy status lofi

# 스트림 중지
youtube-rtsp-proxy stop lofi

# 모든 스트림 중지
youtube-rtsp-proxy stop all
```

### 서버 관리

```bash
# 서버 시작
youtube-rtsp-proxy server start

# 포그라운드에서 실행 (로그 확인용)
youtube-rtsp-proxy server start --foreground

# 서버 재시작
youtube-rtsp-proxy server restart

# 서버 중지
youtube-rtsp-proxy server stop

# 서버 상태 확인
youtube-rtsp-proxy status
```

## 설정

### 설정 파일 위치

우선순위 순:
1. `--config` 플래그로 지정한 경로
2. `/etc/youtube-rtsp-proxy/config.yaml` (시스템)
3. `~/.config/youtube-rtsp-proxy/config.yaml` (사용자)
4. 현재 디렉토리의 `config.yaml`

### 설정 예제

```yaml
# config.yaml
server:
  rtsp_port: 8554
  api_port: 9997

mediamtx:
  binary_path: "mediamtx"
  log_level: "info"

ffmpeg:
  binary_path: "ffmpeg"
  input_options:
    - "-reconnect"
    - "1"
    - "-reconnect_streamed"
    - "1"
  output_options:
    - "-c:v"
    - "copy"
    - "-c:a"
    - "aac"

ytdlp:
  binary_path: "yt-dlp"
  timeout: "30s"
  format: "best[protocol=https]/best"

monitor:
  health_check_interval: "30s"
  url_refresh_interval: "30m"
  max_consecutive_errors: 3
  reconnect:
    initial_delay: "5s"
    max_delay: "5m"
    multiplier: 2.0
    max_attempts: 10

logging:
  level: "info"
  format: "text"
```

### 환경 변수

설정은 환경 변수로도 지정할 수 있습니다 (`YTRTSP_` 접두사):

```bash
export YTRTSP_SERVER_RTSP_PORT=8554
export YTRTSP_SERVER_API_PORT=9997
export YTRTSP_MONITOR_HEALTH_CHECK_INTERVAL=30s
export YTRTSP_MONITOR_URL_REFRESH_INTERVAL=30m
```

## 모니터링 기능

### 자동 URL 갱신

YouTube 라이브 스트림의 URL은 시간이 지나면 만료됩니다. 이 도구는 다음 조건에서 URL을 자동으로 갱신합니다:

| 트리거 | 기본값 | 설명 |
|--------|--------|------|
| 주기적 갱신 | 30분 | 일정 시간마다 선제적 갱신 |
| 연속 실패 | 3회 | 헬스체크 연속 실패 시 즉시 갱신 |
| 에러 감지 | 즉시 | 403, 404 등 URL 관련 에러 시 |

### 자동 재연결

스트림이 끊어지면 자동으로 재연결을 시도합니다:

- **알고리즘**: Exponential Backoff (5초 → 10초 → 20초 ... 최대 5분)
- **최대 시도**: 10회 (설정 가능)
- **URL 갱신**: 필요시 자동으로 새 URL 추출 후 재연결

### 헬스체크 항목

1. FFmpeg 프로세스 생존 확인
2. MediaMTX API를 통한 스트림 상태 확인
3. 데이터 흐름 확인 (수신 바이트 변화 감지)

## 명령어 레퍼런스

### start

YouTube 스트림을 RTSP로 프록시 시작

```
youtube-rtsp-proxy start <youtube-url> [flags]

Flags:
  -n, --name string   스트림 이름 (RTSP 경로로 사용) (기본값: "stream")
  -p, --port int      RTSP 포트 (기본값: 설정 파일의 값)
```

### stop

스트림 중지

```
youtube-rtsp-proxy stop <stream-name|all>
```

### list

활성 스트림 목록 표시

```
youtube-rtsp-proxy list
```

### status

서버 또는 스트림 상태 표시

```
youtube-rtsp-proxy status [stream-name]
```

### server

MediaMTX 서버 제어

```
youtube-rtsp-proxy server <start|stop|restart>

Flags:
  -f, --foreground   포그라운드에서 실행
```

## 프로젝트 구조

```
youtube-rtsp-proxy/
├── cmd/youtube-rtsp-proxy/     # 애플리케이션 진입점
├── internal/
│   ├── cli/                    # Cobra CLI 명령어
│   ├── config/                 # Viper 설정 관리
│   ├── extractor/              # yt-dlp 래퍼
│   ├── stream/                 # 스트림/FFmpeg 관리
│   ├── server/                 # MediaMTX 서버 관리
│   ├── monitor/                # 헬스체크/자동 재연결
│   └── storage/                # 상태 영속화
├── configs/                    # 설정 예제
├── scripts/                    # 설치 스크립트
├── Makefile                    # 빌드 자동화
└── README.md
```

## 문제 해결

### 스트림이 시작되지 않음

```bash
# 의존성 확인
which ffmpeg yt-dlp mediamtx

# 상세 로그와 함께 실행
youtube-rtsp-proxy server start --foreground
```

### PATH에서 명령어를 찾을 수 없음

```bash
# 사용자 설치의 경우
source ~/.bashrc

# PATH 확인
echo $PATH | grep -E "\.local/bin|youtube-rtsp-proxy"
```

### MediaMTX 서버가 이미 실행 중

```bash
# 기존 프로세스 확인
pgrep -f mediamtx

# 서버 재시작
youtube-rtsp-proxy server restart
```

## 라이선스

MIT License

## 기여

버그 리포트, 기능 제안, Pull Request를 환영합니다.
