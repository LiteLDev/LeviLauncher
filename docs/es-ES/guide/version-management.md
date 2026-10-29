# Gestión de versiones

LeviLauncher trata cada entorno instalado como una versión que se puede gestionar de forma independiente. En la aplicación, las versiones instaladas se llaman instancias y aparecen en la página **Instancias**.

## Qué puedes hacer

=== "Windows"

    - Instalar versiones oficiales o de vista previa
    - Asignar a la versión un nombre fácil de identificar
    - Iniciar rápidamente una versión específica
    - Eliminar versiones que ya no necesites
    - Abrir la configuración específica de la versión
    - Crear un acceso directo en el escritorio para la versión

=== "Android"

    - importar APK oficiales de Minecraft y ejecutarlos dentro del lanzador, sin instalar cada versión en el sistema
    - mantener varias versiones y sus datos separados
    - elegir la versión que inicia la pantalla de inicio

## Diferencias entre versión oficial (Release) y de vista previa (Preview) <Badge platform="windows" />

| Tipo | Para quién | Descripción |
| --- | --- | --- |
| Release (versión oficial) | Juego diario estable | Adecuado para la mayoría de jugadores |
| Preview (versión de vista previa) | Pruebas anticipadas de nuevas funciones | Más propenso a cambios de compatibilidad en mundos, paquetes o Mods |

## Qué es el aislamiento

=== "Windows"

    El aislamiento está disponible para las versiones GDK y para las versiones UWP x64 1.19.70.2 o posteriores. Al activar el aislamiento, LeviLauncher guarda los datos del juego de esa versión en su propia área de trabajo, en lugar de compartir la ubicación de datos predeterminada.

    El aislamiento es adecuado para estas necesidades:

    - Usar combinaciones de contenido diferentes en distintas versiones
    - Probar Mods o la versión de vista previa (Preview) de Minecraft de forma más segura
    - Reducir el riesgo de que un entorno afecte a otro

=== "Android"

    Activa el aislamiento para una versión importada cuando sus mundos, paquetes y ajustes deban mantenerse separados de la versión instalada desde Google Play. Las versiones importadas suelen necesitar aislamiento para iniciarse.

## Operaciones comunes con versiones <Badge platform="windows" />

### Instalar una nueva versión

Completa la instalación en la página **Descargar** y luego vuelve a **Iniciar** para iniciarla.

### Renombrar una versión

En **Instancias** o en **Configuración de instancia**, asigna un nombre descriptivo al entorno, por ejemplo, `Release Survival` o `Preview Test`.

### Eliminar una versión

Elimina versiones que ya no necesites, especialmente entornos de prueba de la versión de vista previa (Preview) de Minecraft que ya hayan cumplido su propósito.

::: warning Confirma antes de eliminar
Eliminar una versión podría borrar el contenido asociado a ese entorno gestionado. Haz primero una copia de seguridad de los mundos importantes.
:::

### Crear un acceso directo en el escritorio

Usa **Crear acceso directo en el escritorio** en el menú junto al botón **Iniciar** para crear un acceso directo a una versión específica, lo que facilita su inicio directo.

## Estrategia recomendada para versiones <Badge platform="windows" />

- Una versión oficial (Release) aislada para el juego diario
- Una versión de vista previa (Preview) aislada para usar solo cuando sea necesario
- Una versión de prueba temporal para Mods o diagnóstico de problemas

## Lecturas relacionadas

- [Descargas y mirrors](./downloads-mirrors)
- [Herramientas para mundos](./world-tools)
- [Actualización y solución de problemas](./update-troubleshooting)
