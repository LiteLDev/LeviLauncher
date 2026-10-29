# 모드 및 통합

LeviLauncher는 "기본 런처" 범위를 넘어서는 모드 관련 작업 흐름을 지원합니다.

## 현재 사용 가능한 기능

=== "Windows"

    - `.zip` 모드 가져오기
    - `.dll` 모드 가져오기
    - 설치된 모드 활성화 또는 비활성화
    - 더 이상 필요 없는 모드 삭제
    - CurseForge 관련 탐색 및 가져오기 프로세스 사용
    - 패키지 설치 프로그램인 [lip](https://github.com/LiteLDev/lip)으로 패키지 설치
    - LeviLamina 등의 로더 작업 흐름과 함께 사용

    ::: warning 이는 고급 영역입니다
    모드, 로더 및 패키지 통합의 호환성 변화는 특히 Minecraft 프리뷰(Preview)에서 더 빠르게 발생할 수 있습니다. 이를 고급 기능으로 간주하고 점진적으로 테스트하세요.
    :::

=== "Android"

    - 네이티브 모드(`.so` 라이브러리)를 게임에 로드
    - 모드를 적용한 환경을 격리된 버전에서 유지

    네이티브 모드를 만들고 싶다면 LeviLamina 문서의 [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/)(영어)를 참고하세요.

## 권장 모드 작업 흐름

1. 깨끗한 격리된 버전부터 시작합니다.
2. 해당 버전이 모드 없이 정상적으로 실행되는지 먼저 확인합니다.
3. 한 번에 하나의 모드 또는 소규모 변경 사항만 추가합니다.
4. 각 변경 사항 후 실행을 다시 테스트합니다.
5. 중요한 월드는 항상 백업 경로를 유지합니다.

## CurseForge <Badge platform="windows" />

런처는 CurseForge 관련 탐색 및 패키지 프로세스를 제공하여 앱 내에서 더 편리하게 적절한 리소스를 찾을 수 있도록 도와줍니다.

다음과 같은 요구 사항에 적합합니다:

- 더 안내적인 검색 경험 필요
- 특정 프로젝트의 호환 파일을 더 빠르게 보고 싶음
- 모든 검색을 런처 외부에서 완료하고 싶지 않음

## lip 및 LeviLamina <Badge platform="windows" />

이러한 통합은 패키지 기반 또는 로더 기반 작업 흐름을 사용하려는 고급 사용자에게 더 적합합니다.

모범 사례:

- 안정적인 정식판(Release) 버전부터 시작
- 한 번에 여러 고위험 요소를 변경하지 않음
- "마지막으로 성공적으로 작동한" 버전 조합을 기록

클라이언트 버전에 LeviLamina를 추가하려면 LeviLamina 문서의 [Install client on Windows](https://docs.levimc.org/LeviLamina/user_guides/install_on_client/)(영어)를 참고하세요.

## 네이티브 모드 <Badge platform="android" />

네이티브 모드는 게임 프로세스 안에서 네이티브 코드를 실행합니다. 신뢰할 수 있는 출처의 모드만 설치하고, 먼저 격리된 버전에서 테스트하세요.

직접 만들려면 LeviLamina 문서의 [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/)(영어)를 따르세요.

## 모드를 추가한 후 오류가 발생하면

- 가장 최근에 추가한 모드를 먼저 비활성화
- 마지막으로 정상적으로 작동한 상태와 비교
- 추가 테스트 전에 중요한 월드를 이동하거나 백업
- 그런 다음 [업데이트 및 문제 해결](./update-troubleshooting)을 계속 참조
