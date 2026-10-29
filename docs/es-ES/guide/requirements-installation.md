# Requisitos e instalación

Esta página explica lo que LeviLauncher necesita antes de poder instalar e iniciar Minecraft Bedrock Edition, en Windows y en Android.

Las versiones UWP requieren el modo de desarrollador de Windows y las dependencias del framework UWP declaradas por el paquete (como Microsoft.VCLibs). Los requisitos de Gaming Services y GameInput que aparecen a continuación se aplican a GDK. El aislamiento de instancias funciona con GDK y con los paquetes UWP x64 de la versión 1.19.70.2 o posterior; las instancias UWP no aisladas comparten el contenido `LocalState` de la aplicación de Windows. Las instancias UWP x64 también admiten la consola y los Mods DLL `preload-native` compatibles, pero la instalación automática de LeviLamina solo está disponible para las versiones GDK compatibles.

## Requisitos del sistema

=== "Windows"

    | Elemento | Requisito |
    | --- | --- |
    | Sistema operativo | Windows 10 o Windows 11 |
    | Versión del juego | Minecraft Bedrock Edition (GDK / UWP) |
    | Licencia | Licencia original vinculada a una cuenta de Microsoft |
    | Red | Necesaria para descargar versiones, obtener metadatos, probar la velocidad de mirrors y comprobar actualizaciones |

=== "Android"

    | Elemento | Requisito |
    | --- | --- |
    | Sistema operativo | Android 9.0 (API 28) o posterior |
    | Arquitectura | ARM64 (arm64-v8a) |
    | Memoria | 1 GB de RAM disponible (se recomiendan 3 GB o más) |
    | Almacenamiento | 2 GB libres para Minecraft y los datos del juego (se recomiendan 5 GB o más) |
    | Juego | Minecraft Bedrock Edition 1.21.80 o posterior, instalado desde Google Play |
    | Licencia | Una copia legítima de Minecraft comprada en Google Play |

## Componentes necesarios de Windows <Badge platform="windows" />

Antes del primer inicio o la instalación, LeviLauncher podría guiarte para instalar componentes que falten.

- **Microsoft Gaming Services** (GDK)
- **Microsoft GameInput** (GDK)
- **WebView2 Runtime**
- **Microsoft Visual C++ Redistributable (x64)**, necesario para el cargador de Mods nativo; LeviLauncher comprueba si está instalado y ofrece instalarlo

La presencia o ausencia de estos componentes depende del estado de tu entorno de Windows.

### Usar un WebView2 Fixed Version Runtime personalizado

Si el WebView2 Runtime Evergreen no está instalado pero gestionas tú mismo un WebView2 Fixed Version Runtime, inicia LeviLauncher con cualquiera de los siguientes métodos.

Indica el directorio en la línea de comandos:

```powershell
LeviLauncher.exe --webview2-runtime-dir "D:\WebView2\FixedRuntime"
```

O bien crea `webview2.json` junto a `LeviLauncher.exe`:

```json
{
  "browserExecutableFolder": "D:\\WebView2\\FixedRuntime"
}
```

Las rutas relativas se resuelven desde el directorio que contiene `webview2.json`. El directorio elegido debe contener directamente `msedgewebview2.exe`. La opción de línea de comandos tiene prioridad sobre el archivo de configuración. Si no se configura ninguna de las dos, LeviLauncher sigue usando el WebView2 Runtime instalado en el sistema.

## Antes de instalar una versión

=== "Windows"

    Completa primero esta lista de comprobación:

    1. Instala Minecraft Bedrock al menos una vez desde Microsoft Store.
    2. Si el estado de la tienda es anómalo, inicia el juego una vez para confirmar que la instalación es completa.
    3. Cierra el juego antes de usar LeviLauncher para instalar o gestionar versiones.

=== "Android"

    1. Instala Minecraft Bedrock Edition desde Google Play en el dispositivo.
    2. Actualízalo a 1.21.80 o posterior: el lanzador rechaza las versiones anteriores.
    3. Asegúrate de tener espacio libre suficiente para las versiones y mundos que quieras conservar.

## Instalar LeviLauncher

=== "Windows"

    ### Opción A: Página de GitHub Releases

    Ideal para usuarios que quieren obtener el instalador directamente desde la página oficial de descargas de LeviLauncher y consultar el registro de cambios.

    1. Abre la página de [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases) de LeviLauncher.
    2. Descarga el programa de instalación.
    3. Ejecútalo y completa el asistente de instalación.

    ### Opción B: Mirror en Lanzou

    Si la velocidad de acceso a GitHub es lenta en tu región, esta opción suele ser más conveniente.

    1. Abre [Lanzou](https://levimc.lanzoue.com/b016ke39hc).
    2. Introduce la contraseña `levi`.
    3. Descarga y ejecuta el programa de instalación localmente.

=== "Android"

    1. Abre la página de [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest) de la versión para Android.
    2. Descarga el APK más reciente.
    3. Permite la instalación desde el navegador o el gestor de archivos que usaste para descargarlo.
    4. Instala el APK y ábrelo (la aplicación aparece como "LeviLauncher").
    5. Acepta el acuerdo del lanzador y concede los permisos que solicita la aplicación.

## Instalar la primera versión gestionada

=== "Windows"

    1. Abre **Descargar** en LeviLauncher.
    2. Selecciona la versión de Minecraft **Lanzamiento** (Release) o **Vista previa** (Preview) que deseas instalar, y **GDK** o **UWP** como tipo de paquete.
    3. Selecciona la entrada de la versión objetivo.
    4. Decide si activar el aislamiento.
    5. Inicia la instalación y espera a que finalice.

=== "Android"

    1. Abre la pantalla de versiones (instancias).
    2. Selecciona la versión instalada desde Google Play o importa un APK oficial de Minecraft.
    3. Activa el aislamiento para una versión importada que deba conservar sus datos por separado.
    4. Vuelve a la pantalla de inicio y pulsa **Iniciar**.

## Estrategia de instalación recomendada <Badge platform="windows" />

### Cuándo elegir la versión oficial (Release)

- Quieres un entorno de juego diario más estable
- Estás desarrollando mundos a largo plazo
- Prefieres que los Mods y paquetes de recursos cambien menos

### Cuándo elegir la versión de vista previa (Preview)

- Quieres probar funciones futuras con antelación
- Aceptas inestabilidad o cambios de compatibilidad
- Estás dispuesto a mantener el entorno de vista previa (Preview) separado del entorno de juego diario

::: tip Práctica recomendada para la mayoría de jugadores
Primero crea una **versión oficial (Release) aislada**. Solo añade una **versión de vista previa (Preview)** adicional si necesitas explícitamente probar contenido de vista previa.
:::

## Si la instalación no puede continuar

=== "Windows"

    Los siguientes problemas pueden consultarse en [Actualización y solución de problemas](./update-troubleshooting):

    - Falta Gaming Services
    - Falta GameInput
    - Estado de licencia o instalación de la tienda incompleto
    - La ruta de destino no es escribible
    - Fallo en la descarga o en el mirror

=== "Android"

    - Si el lanzador indica que falta Minecraft o que no es compatible, instala o actualiza primero la versión de Google Play.
    - Las versiones anteriores a 1.21.80 no son compatibles.
    - Para fallos al iniciar, consulta [Actualizaciones y solución de problemas](./update-troubleshooting).
