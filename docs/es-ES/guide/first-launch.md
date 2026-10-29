# Primer inicio

La primera vez que abres LeviLauncher, la aplicación comprueba que tu dispositivo y Minecraft estén listos para gestionar versiones.

## Qué podrías ver

=== "Windows"

    En el primer inicio, LeviLauncher podría:

    - Comprobar si Gaming Services está disponible (GDK)
    - Comprobar si GameInput está disponible (GDK)
    - Comprobar si está instalado el runtime de Visual C++ que necesita el cargador de Mods nativo
    - Verificar si el entorno de Minecraft Bedrock (GDK / UWP) está listo
    - Mostrar una guía o advertencia si faltan componentes clave

=== "Android"

    En el primer inicio, LeviLauncher puede:

    - pedirte que aceptes el acuerdo del lanzador
    - solicitar los permisos que necesita
    - comprobar que el Minecraft oficial de Google Play (1.21.80 o posterior) está instalado
    - migrar los directorios de datos de una versión anterior del lanzador

## Flujo común del primer inicio

=== "Windows"

    1. Abre LeviLauncher.
    2. Lee la información de guía proporcionada por el lanzador.
    3. Instala o repara los componentes previos que falten.
    4. Vuelve al lanzador y vuelve a comprobar el estado.
    5. Una vez cumplidas las condiciones, entra en la página **Descargar**.

=== "Android"

    1. Abre LeviLauncher.
    2. Acepta el acuerdo y concede los permisos solicitados.
    3. Si el lanzador migra datos antiguos, espera a que termine. No borres el directorio antiguo hasta que tus mundos, paquetes y ajustes vuelvan a estar disponibles.
    4. Continúa a la pantalla de versiones.

## Si se indica que faltan componentes <Badge platform="windows" />

### Gaming Services

Si Gaming Services falta o está dañado, LeviLauncher podría guiarte para instalarlo o repararlo desde Microsoft Store.

### GameInput

Si falta GameInput, instala el redistribuible necesario según las indicaciones.

### No se detecta Minecraft Bedrock

Confirma lo siguiente:

- La cuenta de Microsoft actual tiene licencia del juego
- Ya has instalado el juego al menos una vez desde Microsoft Store en este equipo
- No estás intentando usar el lanzador para omitir el proceso oficial de licencia

## Valores predeterminados recomendados para nuevos usuarios

- Empieza usando una versión oficial (Release) aislada
- Coloca las rutas de descarga y contenido en una ubicación de disco con permisos de escritura
- No importes muchos Mods hasta lograr un primer inicio limpio y exitoso

## Después del primer inicio, se recomienda seguir leyendo

- [Gestión de versiones](./version-management)
- [Gestión de contenido](./content-management)
- [Configuración y personalización](./settings-personalization)
