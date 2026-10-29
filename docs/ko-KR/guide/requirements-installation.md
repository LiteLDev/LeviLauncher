# 시스템 요구사항 및 설치

이 페이지에서는 LeviLauncher가 Windows와 Android에서 Minecraft Bedrock Edition을 설치하고 실행하기 전에 필요한 것을 설명합니다.

UWP 버전에는 Windows 개발자 모드와 패키지가 선언한 UWP 프레임워크 종속성(예: Microsoft.VCLibs)이 필요합니다. 아래의 Gaming Services 및 GameInput 요구 사항은 GDK에 적용됩니다. 인스턴스 격리는 GDK와 1.19.70.2 이상의 UWP x64 패키지에서 작동하며, 격리되지 않은 UWP 인스턴스는 Windows 앱의 `LocalState` 콘텐츠를 공유합니다. UWP x64 인스턴스는 콘솔과 호환되는 `preload-native` DLL 모드도 지원하지만, LeviLamina 자동 설치는 지원되는 GDK 버전에서만 사용할 수 있습니다.

## 시스템 요구 사항

=== "Windows"

    | 항목 | 요구 사항 |
    | --- | --- |
    | 운영 체제 | Windows 10 또는 Windows 11 |
    | 게임 버전 | Minecraft Bedrock Edition (GDK / UWP) |
    | 라이선스 | Microsoft 계정에 연결된 정품 라이선스 |
    | 네트워크 | 버전 다운로드, 메타데이터 가져오기, 미러 속도 테스트 및 업데이트 확인을 위한 네트워크 |

=== "Android"

    | 항목 | 요구 사항 |
    | --- | --- |
    | 운영 체제 | Android 9.0(API 28) 이상 |
    | 아키텍처 | ARM64(arm64-v8a) |
    | 메모리 | 사용 가능한 RAM 1GB(3GB 이상 권장) |
    | 저장 공간 | Minecraft와 게임 데이터용 여유 공간 2GB(5GB 이상 권장) |
    | 게임 | Google Play에서 설치한 Minecraft Bedrock Edition 1.21.80 이상 |
    | 라이선스 | Google Play에서 구매한 정품 Minecraft |

## 필요한 Windows 구성 요소 <Badge platform="windows" />

처음 실행하거나 설치하기 전에, LeviLauncher는 누락된 구성 요소를 설치하도록 안내할 수 있습니다.

- **Microsoft Gaming Services** (GDK)
- **Microsoft GameInput** (GDK)
- **WebView2 Runtime**
- **Microsoft Visual C++ Redistributable (x64)**: 네이티브 모드 로더에 필요합니다. LeviLauncher가 설치 여부를 확인하고 설치를 제안합니다

구체적으로 누락되었는지는 Windows 환경 상태에 따라 다릅니다.

### 사용자 지정 WebView2 Fixed Version Runtime 사용

Evergreen WebView2 Runtime이 설치되어 있지 않지만 WebView2 Fixed Version Runtime을 직접 관리하는 경우, 다음 방법 중 하나로 LeviLauncher를 시작하세요.

명령줄에서 디렉터리를 지정합니다:

```powershell
LeviLauncher.exe --webview2-runtime-dir "D:\WebView2\FixedRuntime"
```

또는 `LeviLauncher.exe` 옆에 `webview2.json`을 만듭니다:

```json
{
  "browserExecutableFolder": "D:\\WebView2\\FixedRuntime"
}
```

상대 경로는 `webview2.json`이 있는 디렉터리를 기준으로 해석됩니다. 선택한 디렉터리에는 `msedgewebview2.exe`가 직접 들어 있어야 합니다. 명령줄 옵션이 구성 파일보다 우선합니다. 둘 다 구성되지 않은 경우 LeviLauncher는 계속 시스템에 설치된 WebView2 Runtime을 사용합니다.

## 버전을 설치하기 전에

=== "Windows"

    다음 체크리스트를 먼저 완료하세요:

    1. Microsoft Store에서 Minecraft Bedrock을 최소한 한 번은 설치했는지 확인합니다.
    2. 스토어 상태가 비정상적인 경우, 게임을 한 번 실행하여 설치가 완전한지 확인합니다.
    3. LeviLauncher를 사용하여 버전을 설치하거나 관리하기 전에 게임을 종료합니다.

=== "Android"

    1. 기기에 Google Play에서 Minecraft Bedrock Edition을 설치합니다.
    2. 1.21.80 이상으로 업데이트합니다. 더 오래된 버전은 런처가 거부합니다.
    3. 보관할 버전과 월드를 위한 저장 공간이 충분한지 확인합니다.

## LeviLauncher 본체 설치

=== "Windows"

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

=== "Android"

    1. Android 버전의 [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest) 페이지를 엽니다.
    2. 최신 APK를 다운로드합니다.
    3. 다운로드에 사용한 브라우저나 파일 관리자에서 설치를 허용합니다.
    4. APK를 설치하고 엽니다(앱 이름은 "LeviLauncher").
    5. 런처 약관에 동의하고 앱이 요청하는 권한을 허용합니다.

## 첫 번째 호스팅 버전 설치

=== "Windows"

    1. LeviLauncher 내에서 **다운로드**를 엽니다.
    2. 설치하려는 Minecraft 빌드로 **릴리스** 또는 **미리보기**를, 패키지 유형으로 **GDK** 또는 **UWP**를 선택합니다.
    3. 대상 버전 항목을 선택합니다.
    4. 격리 사용 여부를 결정합니다.
    5. 설치를 시작하고 완료될 때까지 기다립니다.

=== "Android"

    1. 버전(인스턴스) 화면을 엽니다.
    2. Google Play에서 설치한 버전을 선택하거나 공식 Minecraft APK를 가져옵니다.
    3. 데이터를 분리해야 하는 가져온 버전에는 버전 격리를 켭니다.
    4. 실행 화면으로 돌아가 **실행**을 누릅니다.

## 권장 설치 전략 <Badge platform="windows" />

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

=== "Windows"

    다음 문제는 [업데이트 및 문제 해결](./update-troubleshooting)을 계속 참조할 수 있습니다:

    - Gaming Services 누락
    - GameInput 누락
    - 스토어 라이선스 또는 설치 상태 불완전
    - 대상 경로에 쓰기 불가
    - 다운로드 또는 미러 실패

=== "Android"

    - 런처가 Minecraft가 없거나 지원되지 않는다고 알리면 먼저 Google Play 버전을 설치하거나 업데이트하세요.
    - 1.21.80 미만 버전은 지원되지 않습니다.
    - 실행 실패는 [업데이트 및 문제 해결](./update-troubleshooting)을 참고하세요.
