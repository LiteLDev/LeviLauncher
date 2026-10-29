# 처음 실행하기

LeviLauncher를 처음 열면 앱이 기기와 Minecraft가 버전 관리를 할 준비가 되었는지 확인합니다.

## 어떤 화면을 볼 수 있나요?

=== "Windows"

    첫 실행 시 LeviLauncher는 다음과 같은 작업을 수행할 수 있습니다:

    - Gaming Services가 사용 가능한지 확인 (GDK)
    - GameInput이 사용 가능한지 확인 (GDK)
    - 네이티브 모드 로더에 필요한 Visual C++ 런타임이 설치되어 있는지 확인
    - Minecraft Bedrock (GDK / UWP) 환경이 준비되었는지 검증
    - 핵심 구성 요소가 누락된 경우 안내 또는 알림 표시

=== "Android"

    처음 실행할 때 LeviLauncher는 다음을 할 수 있습니다.

    - 런처 약관 동의 요청
    - 필요한 권한 요청
    - Google Play 공식 Minecraft(1.21.80 이상) 설치 여부 확인
    - 이전 버전 런처의 데이터 디렉터리 마이그레이션

## 일반적인 첫 실행 흐름

=== "Windows"

    1. LeviLauncher를 엽니다.
    2. 런처에서 제공하는 안내 정보를 읽습니다.
    3. 누락된 사전 구성 요소를 설치하거나 복구합니다.
    4. 런처로 돌아가 상태를 다시 확인합니다.
    5. 조건을 충족하면 **다운로드** 페이지로 이동합니다.

=== "Android"

    1. LeviLauncher를 엽니다.
    2. 약관에 동의하고 요청된 권한을 허용합니다.
    3. 런처가 이전 데이터를 마이그레이션하면 끝날 때까지 기다립니다. 월드, 팩, 설정을 다시 사용할 수 있을 때까지 이전 디렉터리를 삭제하지 마세요.
    4. 버전 화면으로 이동합니다.

## 구성 요소가 누락되었다고 표시되면 <Badge platform="windows" />

### Gaming Services

Gaming Services가 누락되었거나 손상된 경우, LeviLauncher는 Microsoft Store에서 설치하거나 복구하도록 안내할 수 있습니다.

### GameInput

GameInput이 누락된 경우, 안내에 따라 필요한 redistributable을 설치하세요.

### Minecraft Bedrock이 감지되지 않음

다음을 확인하세요:

- 현재 Microsoft 계정에 게임 라이선스가 있는지
- 현재 컴퓨터에 Microsoft Store에서 게임을 이미 설치했는지
- 공식 라이선스 프로세스를 우회하려고 런처를 사용하지는 않는지

## 새 사용자를 위한 좋은 기본값

- 격리된 정식판(Release) 버전부터 시작
- 다운로드 및 콘텐츠 경로를 쓰기 가능한 디스크 위치에 배치
- 첫 번째 성공적인 깨끗한 실행 전에는 많은 모드를 서두르지 말 것

## 첫 실행 후 권장 읽기 자료

- [버전 관리](./version-management)
- [콘텐츠 관리](./content-management)
- [설정 및 개인화](./settings-personalization)
