# Actualizaciones y solución de problemas

Cuando el comportamiento de instalación, actualización o inicio de LeviLauncher no sea el esperado, consulta primero esta página.

## Problemas comunes <Badge platform="windows" />

### Gaming Services ausente o dañado

- Utiliza la guía proporcionada por el lanzador para instalarlo o repararlo.
- Si es necesario, vuelve a Microsoft Store para comprobar el estado de la instalación de Minecraft Bedrock.

### GameInput ausente

- Instala el redistribuible necesario según las indicaciones de LeviLauncher.
- Tras la instalación, reinicia el lanzador si es necesario.

### Ruta de instalación no escribible

- En **Configuración → Configuración básica**, cambia la ruta del contenido gestionado a una ubicación escribible.
- Si el lanzador solicita privilegios elevados durante la instalación o autoactualización, solo permítelo si confías en la fuente.

### LeviLauncher se cierra al instante o nunca muestra una ventana

- En Windows, LeviLauncher ahora inicia WebView2 en un modo de renderizado por software más seguro para reducir los cierres inesperados al arrancar con algunas combinaciones de GPU y controladores.
- Si el lanzador sigue fallando antes de que aparezca la interfaz, revisa `%APPDATA%\levilauncher.exe\logs\startup.log`.
- Vuelve a intentarlo una vez tras un inicio fallido. Si sigue ocurriendo, adjunta `startup.log` al abrir un Issue en GitHub.

### Una versión no se inicia

- Primero prueba si la versión se inicia sin Mods
- Comprueba si los componentes necesarios de Windows siguen presentes
- Confirma que la versión se ha instalado completamente
- Intenta reproducir el problema en un entorno de prueba aislado y limpio

### Descarga muy lenta o fallos repetidos

- Cambia de mirror
- Reinténtalo más tarde
- Si es posible, utiliza una fuente de paquetes locales

## Problemas comunes en Android <Badge platform="android" />

### Falta Minecraft o no es compatible

Instala o actualiza la aplicación oficial de Minecraft desde Google Play (1.21.80 o posterior) y vuelve a abrir LeviLauncher.

### Una versión importada no se inicia

Las versiones importadas suelen necesitar aislamiento. Actívalo en los ajustes de la instancia y vuelve a intentarlo.

### Un inicio falla

Abre el registro de inicio desde la pantalla de error y revisa el motivo antes de cambiar ajustes.

### Migración de datos

El lanzador puede migrar directorios de datos antiguos antes de continuar. No borres el directorio antiguo hasta que la migración termine y tus mundos, paquetes y ajustes estén disponibles.

## Pasos de recuperación más seguros

1. Haz primero una copia de seguridad de los mundos importantes.
2. Elimina los Mods o paquetes de recursos añadidos recientemente.
3. Realiza una prueba de comparación con una versión aislada limpia.
4. Si es necesario, reinstala la versión afectada.

## Sobre la autoactualización <Badge platform="windows" />

LeviLauncher puede comprobar, descargar e instalar actualizaciones de la aplicación. Si el directorio de instalación no es escribible, en algunos entornos podría requerir permisos de administrador.

## Cuándo deberías crear un Issue

Se recomienda enviar un problema a GitHub cuando se cumplan las siguientes condiciones:

- Puedes reproducir el problema de forma estable
- El problema persiste tras realizar los pasos básicos de recuperación
- El problema parece ser de LeviLauncher en sí, y no un problema general del entorno de Windows

Al enviarlo, incluye siempre:

- Versión de Windows
- Versión de LeviLauncher
- Tus pasos de operación específicos
- Capturas de pantalla o registros (si los tienes)

Enlace para informar de problemas: [GitHub Issues](https://github.com/LiteLDev/LeviLauncher/issues)

Para la versión para Android, incluye la versión de Android, el modelo del dispositivo y el registro de inicio, y abre la incidencia en [LiteLDev/LeviLaunchroid](https://github.com/LiteLDev/LeviLaunchroid/issues).
