# Comienza en 5 minutos

Si solo quieres el camino más corto desde la descarga hasta jugar, sigue primero esta página. Elige **Windows** o **Android** en cualquier pestaña: toda la guía sigue tu elección.

## 1. Instalar LeviLauncher

=== "Windows"

    - Descarga el instalador de LeviLauncher desde la página de [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases) o desde el [mirror en Lanzou](https://levimc.lanzoue.com/b016ke39hc). La contraseña de Lanzou es `levi`.
    - Ejecuta el programa de instalación en Windows.
    - Si el proceso de instalación indica que necesita WebView2 u otras dependencias, permite que continúe.

=== "Android"

    - Descarga el APK más reciente desde [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest).
    - Permite instalar aplicaciones desde el navegador o el gestor de archivos que usaste e instala el APK.
    - Abre LeviLauncher y acepta el acuerdo del lanzador y los permisos que solicita.

## 2. Prepara Minecraft

=== "Windows"

    - Inicia sesión con una cuenta de Microsoft que tenga **Minecraft Bedrock Edition**.
    - Asegúrate de haber instalado el juego al menos una vez desde Microsoft Store en este equipo.
    - Confirma que **Gaming Services** está disponible (necesario para las versiones GDK).

    ::: warning Licencia original obligatoria
    LeviLauncher se utiliza para gestionar Minecraft Bedrock (GDK / UWP), pero no puede sustituir la licencia del juego.
    :::

=== "Android"

    - Instala **Minecraft Bedrock Edition** 1.21.80 o posterior desde **Google Play** en este dispositivo.
    - LeviLauncher usa esta copia oficial como fuente de confianza del juego; no sustituye la licencia del juego.

## 3. Elige una versión

=== "Windows"

    - Inicia LeviLauncher.
    - Entra en **Descargar** desde la barra lateral.
    - Selecciona **Lanzamiento** o **Vista previa**, y un paquete **GDK** o **UWP**.
    - Aquí, **Lanzamiento** y **Vista previa** se refieren a la versión oficial (Release) y a la versión de vista previa (Preview) de Minecraft.
    - Selecciona la versión y comienza la instalación.

=== "Android"

    - Ve a la pantalla de versiones (instancias).
    - Selecciona la versión de Minecraft que quieres ejecutar o importa un APK oficial de Minecraft para ejecutar otra versión dentro del lanzador.

## 4. Decidir si activar el aislamiento

=== "Windows"

    El aislamiento coloca los datos del juego de esa versión en un área de trabajo independiente, en lugar de mezclarlos con el entorno AppData predeterminado. Está disponible para las versiones GDK y para las versiones UWP x64 1.19.70.2 o posteriores.

    Se recomienda activar el aislamiento en estos escenarios:

    - Pruebas de Mods sin afectar al entorno principal
    - Separar la versión oficial de Minecraft de la de vista previa
    - Reservar mundos y paquetes de recursos diferentes para distintos propósitos de juego

=== "Android"

    Activa el aislamiento de versiones al iniciar una versión importada cuyos mundos, paquetes y ajustes deban mantenerse separados de la versión instalada desde Google Play. Las versiones importadas suelen necesitarlo para iniciarse.

## 5. Iniciar la versión instalada

=== "Windows"

    - Vuelve a **Iniciar**.
    - Selecciona la versión recién instalada.
    - Haz clic en **Iniciar**.

    Si LeviLauncher indica que faltan GameInput o Gaming Services, completa primero la instalación de las dependencias según las indicaciones.

=== "Android"

    - Vuelve a la pantalla de inicio y pulsa **Iniciar**.
    - Si el inicio falla, abre el registro de inicio desde la pantalla de error y revisa el motivo antes de cambiar ajustes.

## 6. Continuar leyendo las secciones correspondientes

- Para preparar el entorno: [Requisitos e instalación](./requirements-installation)
- Para saber qué ocurre al entrar por primera vez en la aplicación: [Primer inicio](./first-launch)
- Para organizar versiones: [Gestión de versiones](./version-management)
- Para gestionar mundos, paquetes de recursos o capturas: [Gestión de contenido](./content-management)
- Para usar Mods o fuentes de integración: [Mods e integraciones](./mods-integrations)
