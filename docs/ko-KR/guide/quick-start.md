# 5분 만에 시작하기

다운로드부터 플레이까지 가장 빠른 방법만 알고 싶다면 이 페이지부터 따라 하세요. 아무 탭에서나 **Windows** 또는 **Android**를 선택하면 가이드 전체가 그 선택을 따릅니다.

## 1. LeviLauncher 설치

=== "Windows"

    - [GitHub 릴리스](https://github.com/LiteLDev/LeviLauncher/releases) 페이지 또는 [란조우(Lanzou) 클라우드 미러](https://levimc.lanzoue.com/b016ke39hc)에서 LeviLauncher 설치 패키지를 다운로드하세요. 란조우 비밀번호는 `levi`입니다.
    - Windows에서 설치 프로그램을 실행합니다.
    - 설치 과정에서 WebView2 또는 기타 종속성이 필요하다는 메시지가 표시되면 계속 설치하도록 허용하세요.

=== "Android"

    - [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest)에서 최신 APK를 다운로드합니다.
    - 사용한 브라우저나 파일 관리자에서 앱 설치를 허용한 뒤 APK를 설치합니다.
    - LeviLauncher를 열고 런처 약관과 요청하는 권한을 허용합니다.

## 2. Minecraft 준비하기

=== "Windows"

    - **Minecraft Bedrock Edition** 이 있는 Microsoft 계정으로 로그인합니다.
    - 현재 컴퓨터에서 Microsoft Store를 통해 게임을 최소한 한 번은 설치했는지 확인합니다.
    - **Gaming Services**가 사용 가능한지 확인합니다(GDK 버전에 필요).

    ::: warning 정품이 필요합니다
    LeviLauncher는 Minecraft Bedrock (GDK / UWP)을 관리하기 위한 것이며, 게임 라이선스를 대체할 수 없습니다.
    :::

=== "Android"

    - 이 기기에 **Google Play**에서 **Minecraft Bedrock Edition** 1.21.80 이상을 설치합니다.
    - LeviLauncher는 이 공식 게임을 신뢰할 수 있는 게임 출처로 사용하며, 게임 라이선스를 대신하지 않습니다.

## 3. 버전 선택하기

=== "Windows"

    - LeviLauncher를 실행합니다.
    - 사이드바에서 **다운로드**로 이동합니다.
    - **릴리스** 또는 **미리보기** 빌드를 선택하고, **GDK** 또는 **UWP** 패키지를 선택합니다.
    - 여기서 **릴리스**와 **미리보기**는 Minecraft의 정식판과 프리뷰판을 의미합니다.
    - 버전을 선택하고 설치를 시작합니다.

=== "Android"

    - 버전(인스턴스) 화면으로 이동합니다.
    - 실행할 Minecraft 버전을 선택하거나, 공식 Minecraft APK를 가져와 런처 안에서 다른 버전을 실행합니다.

## 4. 격리 사용 여부 결정

=== "Windows"

    격리를 활성화하면 해당 버전의 게임 데이터가 기본 AppData 환경과 분리된 독립 작업 영역에 저장됩니다. 격리는 GDK 버전과 1.19.70.2 이상의 UWP x64 버전에서 사용할 수 있습니다.

    다음 시나리오에서는 격리를 활성화하는 것이 좋습니다:

    - 모드를 테스트하되 메인 환경에 영향을 주고 싶지 않을 때
    - Minecraft 정식판과 프리뷰판을 분리하고 싶을 때
    - 다른 플레이 목적을 위해 서로 다른 월드와 리소스 팩을 보관하고 싶을 때

=== "Android"

    가져온 버전의 월드, 팩, 설정을 Google Play에서 설치한 버전과 분리하려면 실행할 때 버전 격리를 켜세요. 가져온 버전은 실행하려면 격리가 필요한 경우가 많습니다.

## 5. 설치된 버전 실행

=== "Windows"

    - **실행** 페이지로 돌아갑니다.
    - 방금 설치한 버전을 선택합니다.
    - **실행**을 클릭합니다.

    LeviLauncher가 GameInput 또는 Gaming Services가 누락되었다고 보고하면, 안내에 따라 종속성을 먼저 설치하세요.

=== "Android"

    - 실행 화면으로 돌아가 **실행**을 누릅니다.
    - 실행에 실패하면 설정을 바꾸기 전에 실패 화면에서 실행 로그를 열어 원인을 확인하세요.

## 6. 관련 섹션 계속 읽기

- 환경 준비 필요: [시스템 요구 사항 및 설치](./requirements-installation)
- 앱을 처음 실행할 때 어떤 일이 발생하는지 알고 싶음: [첫 실행](./first-launch)
- 버전 정리 필요: [버전 관리](./version-management)
- 월드, 리소스 팩 또는 스크린샷 관리 필요: [콘텐츠 관리](./content-management)
- 모드 또는 패키지 소스 사용 필요: [모드 및 통합](./mods-integrations)
