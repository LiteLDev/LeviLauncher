# Mods e integraciones

LeviLauncher admite flujos de trabajo relacionados con Mods que van más allá de un "lanzador básico".

## Capacidades actuales

=== "Windows"

    - Importar paquetes de Mods (`.mcmod`)
    - Los paquetes `.zip` y los Mods `.dll` sueltos se siguen aceptando: elige **Todos los archivos (\*.\*)** en el selector de archivos
    - Activar o desactivar Mods instalados
    - Eliminar Mods que ya no necesites
    - Utilizar flujos de navegación y obtención relacionados con CurseForge
    - Instalar paquetes LIP con el gestor de paquetes integrado, compatible con [lip](https://github.com/LiteLDev/lip)
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

LeviLauncher incluye un gestor de paquetes integrado compatible con [lip](https://github.com/LiteLDev/lip), así que no necesitas instalar lip ni el runtime de .NET por separado. Sigue lip: cada versión registra sus paquetes en `.lip/state.json` y los paquetes se leen de su `lip.toml` (los paquetes antiguos que solo tienen un `tooth.json` siguen funcionando). Las versiones que ya tienen LeviLamina o paquetes LIP instalados con un launcher anterior (`tooth_lock.json`) se migran automáticamente en la primera operación de paquetes. Con **Usar servidores espejo** activado, las descargas de paquetes desde GitHub pasan por el **Proxy de GitHub** configurado en [Ajustes](./settings-personalization); si no, se conectan directamente a GitHub (a través del proxy del sistema, si hay uno configurado). Un proxy de GitHub guardado anteriormente, o un `github_proxy` definido en el `config.toml` de lip (en `%APPDATA%\lip`, por ejemplo con `lip config set github_proxy <URL>`), se aplica en ambos casos. **Limpiar caché** elimina los archivos de paquetes descargados.

Desde lip v0.40, cada entrada de un `lip.toml` y de `.lip/state.json` indica dónde se ejecuta con `side` (`server` o `client`) y `platform` (un identificador de runtime de .NET como `win-x64`); lip v0.39 y anteriores las llamaban `platform` y `runtime`. Un `.lip/state.json` escrito con las claves antiguas, por lip v0.39 o anterior o por un lanzador antiguo, se convierte automáticamente antes de la siguiente operación de paquetes: `platform` pasa a `side` y `runtime` a `platform`. Un archivo que lip siga sin poder leer se rechaza con un mensaje que indica el cambio: renombra `platform` a `side` y `runtime` a `platform`, o reinstala los paquetes de esa versión.

Este tipo de integración es más adecuado para usuarios avanzados que desean utilizar flujos de trabajo basados en paquetes o cargadores.

Mejores prácticas:

- Empieza siempre con una versión oficial (Release) estable
- No cambies varios factores de alto riesgo a la vez
- Registra la combinación de versión que funcionó "por última vez"

Para añadir LeviLamina a una versión de cliente, consulta [Install client on Windows](https://docs.levimc.org/LeviLamina/user_guides/install_on_client/) en la documentación de LeviLamina (en inglés).

### Página LeviMC Mods

La página **LeviMC Mods** muestra los mods de [levimc.org/mods](https://levimc.org/mods/) que tienen una compilación para el cliente en Windows x64 (una entrada con `side = "client"` y `platform = "win-x64"`). Puedes buscarlos por nombre, ID, autor, etiqueta o descripción; ordenarlos por relevancia, estrellas, última actualización o más recientes; y filtrarlos por versión del juego, versión de LeviLamina, etiquetas (las recomendadas, como Biblioteca / API o Economía, van primero, y una etiqueta también se puede excluir), licencia (una familia como Permisiva o un ID SPDX), dependencias y autor (**Más de …** en la página de un mod). Al instalar un mod se elige su variante `client`, si no la variante predeterminada, y si no la primera otra variante con compilación para el cliente; en la página de un mod puedes elegir otra de esas variantes.

## Mods nativos <Badge platform="android" />

Los Mods nativos ejecutan código nativo en el proceso del juego. Instala solo Mods de fuentes de confianza y pruébalos primero en una versión aislada.

Para crear los tuyos, sigue [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) en la documentación de LeviLamina (en inglés).

## Si aparecen errores tras añadir Mods

- Desactiva primero el Mod más reciente
- Compara con el estado anterior en el que funcionaba correctamente
- Antes de seguir probando, transfiere o haz una copia de seguridad de los mundos importantes
- Luego, consulta [Actualización y solución de problemas](./update-troubleshooting)
