# 시스템 요구 사항 및 설치

이 페이지는 LeviLauncher가 Minecraft Bedrock (GDK) 버전을 정상적으로 설치 및 관리하기 전에 충족해야 하는 조건을 설명합니다.

## 시스템 요구 사항

| 항목 | 요구 사항 |
| --- | --- |
| 운영 체제 | Windows 10 또는 Windows 11 |
| 게임 버전 | Minecraft Bedrock Edition (GDK) |
| 라이선스 | Microsoft 계정에 연결된 정품 라이선스 |
| 네트워크 | 버전 다운로드, 메타데이터 가져오기, 미러 속도 테스트 및 업데이트 확인을 위한 네트워크 |

## 필요한 Windows 구성 요소

처음 실행하거나 설치하기 전에, LeviLauncher는 누락된 구성 요소를 설치하도록 안내할 수 있습니다.

- **Microsoft Gaming Services**
- **Microsoft GameInput**
- **WebView2 Runtime**

구체적으로 누락되었는지는 Windows 환경 상태에 따라 다릅니다.

## 버전을 설치하기 전에

다음 체크리스트를 먼저 완료하세요:

1. Microsoft Store에서 Minecraft Bedrock을 최소한 한 번은 설치했는지 확인합니다.
2. 스토어 상태가 비정상적인 경우, 게임을 한 번 실행하여 설치가 완전한지 확인합니다.
3. LeviLauncher를 사용하여 버전을 설치하거나 관리하기 전에 게임을 종료합니다.

## LeviLauncher 본체 설치

### 방법 A: GitHub 릴리스 페이지

LeviLauncher 공식 다운로드 페이지에서 설치 패키지를 직접 가져오고, 동시에 업데이트 기록을 확인하고 싶은 사용자에게 적합합니다.

1. LeviLauncher의 [GitHub 릴리스](https://github.com/LiteLDev/LeviLauncher/releases) 페이지를 엽니다.
2. 설치 프로그램을 다운로드합니다.
3. 실행하여 설치 마법사를 완료합니다.

### 방법 B: 란조우(Lanzou) 클라우드 미러

귀하의 지역에서 GitHub 접속 속도가 느릴 경우, 이 진입점이 일반적으로 더 편리합니다.

1. [란조우(Lanzou)](https://levimc.lanzoue.com/b016ke39hc)를 엽니다.
2. 비밀번호 `levi`를 입력합니다.
3. 다운로드 후 로컬에서 설치 프로그램을 실행합니다.

## 첫 번째 호스팅 버전 설치

1. LeviLauncher 내에서 **Download**를 엽니다.
2. 설치하려는 Minecraft **Release** 또는 **Preview**를 선택합니다.
3. 대상 버전 항목을 선택합니다.
4. 격리 사용 여부를 결정합니다.
5. 설치를 시작하고 완료될 때까지 기다립니다.

## 권장 설치 전략

### 언제 정식판(Release)을 선택하나요?

- 더 안정적인 일상 플레이 환경을 원할 때
- 장기 월드를 운영 중일 때
- 모드 및 리소스 팩의 변경 사항이 적기를 원할 때

### 언제 프리뷰판(Preview)을 선택하나요?

- 미래 기능을 사전에 체험하고 싶을 때
- 불안정성이나 호환성 변경을 감수할 수 있을 때
- Minecraft 프리뷰(Preview) 환경을 일상 플레이 환경과 분리하고 싶을 때

::: tip 대부분의 플레이어를 위한 권장 사항
먼저 **격리된 정식판(Release) 버전**을 만드세요. 명확하게 프리뷰 콘텐츠를 체험해야 할 때만 추가로 **프리뷰판(Preview)** 을 추가하세요.
:::

## 설치가 계속되지 않으면

다음 문제는 [업데이트 및 문제 해결](./update-troubleshooting)을 계속 참조할 수 있습니다:

- Gaming Services 누락
- GameInput 누락
- 스토어 라이선스 또는 설치 상태 불완전
- 대상 경로에 쓰기 불가
- 다운로드 또는 미러 실패