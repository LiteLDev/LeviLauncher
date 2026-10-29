# Mods e integraciones

LeviLauncher admite flujos de trabajo relacionados con Mods que van más allá de un "lanzador básico".

## Capacidades actuales

=== "Windows"

    - Importar Mods `.zip`
    - Importar Mods `.dll`
    - Activar o desactivar Mods instalados
    - Eliminar Mods que ya no necesites
    - Utilizar flujos de navegación y obtención relacionados con CurseForge
    - Instalar paquetes con [lip](https://github.com/LiteLDev/lip), el instalador de paquetes
    - Trabajar con flujos de cargadores como LeviLamina

    ::: warning Esta es un área avanzada
    La compatibilidad de Mods, cargadores e integraciones de paquetes suele cambiar más rápido, especialmente en la versión de vista previa (Preview) de Minecraft. Considera esto como una capacidad avanzada y prueba paso a paso.
    :::

=== "Android"

    - cargar Mods nativos (bibliotecas `.so`) en el juego
    - mantener configuraciones con Mods en versiones aisladas

    ¿Quieres crear Mods nativos? Consulta [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) en la documentación de LeviLamina (en inglés).

## Flujo de trabajo recomendado para Mods

1. Empieza con una versión aislada limpia.
2. Confirma primero que dicha versión se inicia correctamente sin Mods.
3. Añade solo un Mod o un pequeño grupo de cambios a la vez.
4. Tras cada cambio, vuelve a probar el inicio.
5. Conserva siempre una ruta de copia de seguridad para los mundos importantes.

## CurseForge <Badge platform="windows" />

El lanzador ofrece flujos de navegación y paquetes de CurseForge para ayudarte a encontrar recursos adecuados de forma más cómoda dentro de la aplicación.

Ideal para estas necesidades:

- Necesitas una experiencia de búsqueda más guiada
- Quieres ver rápidamente los archivos compatibles de un proyecto
- Prefieres no realizar todas las búsquedas fuera del lanzador

## lip y LeviLamina <Badge platform="windows" />

Este tipo de integración es más adecuado para usuarios avanzados que desean utilizar flujos de trabajo basados en paquetes o cargadores.

Mejores prácticas:

- Empieza siempre con una versión oficial (Release) estable
- No cambies varios factores de alto riesgo a la vez
- Registra la combinación de versión que funcionó "por última vez"

Para añadir LeviLamina a una versión de cliente, consulta [Install client on Windows](https://docs.levimc.org/LeviLamina/user_guides/install_on_client/) en la documentación de LeviLamina (en inglés).

## Mods nativos <Badge platform="android" />

Los Mods nativos ejecutan código nativo en el proceso del juego. Instala solo Mods de fuentes de confianza y pruébalos primero en una versión aislada.

Para crear los tuyos, sigue [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) en la documentación de LeviLamina (en inglés).

## Si aparecen errores tras añadir Mods

- Desactiva primero el Mod más reciente
- Compara con el estado anterior en el que funcionaba correctamente
- Antes de seguir probando, transfiere o haz una copia de seguridad de los mundos importantes
- Luego, consulta [Actualización y solución de problemas](./update-troubleshooting)
