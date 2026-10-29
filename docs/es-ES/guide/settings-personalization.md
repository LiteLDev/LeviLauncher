---
platforms: [windows]
---
# Ajustes y personalización

LeviLauncher ofrece una página de configuración bastante completa para ajustar rutas, apariencia, gestión de componentes, preferencias de privacidad y estrategia de actualizaciones.

## Principales secciones de configuración

Actualmente, la aplicación divide la configuración en varias áreas, por ejemplo:

- Configuración básica
- Personalización
- Componentes de terceros
- Otros
- Privacidad
- Actualizaciones
- Acerca de

## Qué puedes ajustar

### Rutas y almacenamiento

- Ruta base del contenido
- Ubicación escribible para los archivos gestionados
- Forma de colocar el entorno que se adapte mejor a tu disposición de discos

### Apariencia

- Idioma
- Preferencia de tema claro u oscuro
- Comportamiento del color del tema
- Fondo y elementos de personalización visual

#### Fondo de pantalla y material de la interfaz

Elige una carpeta de imágenes en **Personalización → Fondo de pantalla → Imagen de fondo** y luego abre **Personalización → Materiales y legibilidad** para ajustar el **Material de la interfaz**, la capa situada entre el fondo de pantalla y el contenido. La navegación, las tarjetas, las pestañas, los controles de formulario y las superficies emergentes comparten un mismo sistema de material. Los perfiles claro y oscuro se guardan por separado y se aplican automáticamente según el tema de la aplicación; los botones **Modo claro** / **Modo oscuro** del editor de material solo seleccionan el perfil que se edita y se previsualiza.

| Ajuste | Comportamiento |
| --- | --- |
| Equilibrado | Ajuste predeterminado: cristal suave con la protección de legibilidad activada |
| Nítido | El fondo de pantalla se ve más a través de las tarjetas, con la protección de legibilidad desactivada; funciona mejor con imágenes sencillas y de bajo contraste |
| Sólido | Tarjetas opacas sin desenfoque de tarjeta; el fondo de pantalla sigue visible fuera de las tarjetas |
| Opacidad de la interfaz | Cambia el tinte neutro de las superficies de la interfaz sin atenuar el texto, los iconos ni los botones |
| Desenfoque de la interfaz | Controla el desenfoque detrás de los paneles, la navegación y las superficies emergentes; el desenfoque de imagen afecta a todo el fondo de pantalla |
| Capa sobre el fondo | Suaviza el fondo de pantalla con un tinte claro en el modo claro y un tinte oscuro en el modo oscuro, de forma independiente del brillo y la opacidad de la imagen |
| Protección de legibilidad | Aplica un tinte mínimo conservador según el tema, el brillo, la opacidad y el desenfoque de la imagen, y refuerza el texto secundario |

La vista previa en directo comparte la imagen cargada y los parámetros de renderizado con la ventana principal. Cuando la protección aumenta la opacidad efectiva de las tarjetas, el editor muestra el valor aplicado. Este cálculo no analiza ni sube la imagen. **Restablecer este tema** solo restaura el perfil de material que se está editando; no borra la carpeta de imágenes ni el perfil del otro tema.

Usa **Siguiente imagen** cuando la carpeta contenga varias imágenes. El orden **Secuencial** usa la ordenación natural de los nombres de archivo; el orden **Aleatorio** evita repetir la imagen actual. Si no hay ninguna imagen seleccionada, la imagen no se puede cargar o su opacidad es 0, la interfaz vuelve a las superficies normales. El contraste alto, la transparencia reducida y los filtros de fondo no compatibles usan superficies sólidas de reserva.

### Componentes de terceros

- Estado de [lip](https://github.com/LiteLDev/lip): comprobarlo, instalarlo o actualizarlo, y limpiar su caché

### Privacidad

- Estadísticas de uso anónimas (Microsoft Clarity), que puedes desactivar en cualquier momento
- Enlaces a los términos de Clarity y a la declaración de privacidad de Microsoft

### Actualizaciones

- Comprobación de actualizaciones
- **Recibir actualizaciones Beta** para obtener versiones preliminares del lanzador

## Valores predeterminados recomendados

- Coloca los archivos gestionados en una ubicación de disco escribible y con espacio suficiente
- Deja **Recibir actualizaciones Beta** desactivado, salvo que quieras versiones preliminares del lanzador
- Estabiliza primero el entorno básico antes de ajustar minuciosamente la apariencia

## Cuándo merece la pena revisar la configuración

Te recomendamos revisar la página de configuración en estos casos:

- Quieres migrar la instalación a otro disco
- Encuentras errores de permisos de escritura
- Quieres usar una apariencia o estrategia de rutas diferente en distintas máquinas
- Necesitas diagnosticar problemas de descarga o reconocimiento de componentes
