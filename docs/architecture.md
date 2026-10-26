# Arquitectura

## Límites

`main.go` compone servicios y recursos nativos. React se comunica exclusivamente mediante los bindings generados de `internal/desktop.Service`. El frontend no escribe archivos ni modifica variables por su cuenta.

`internal/manager` implementa las reglas de JDKs y la persistencia sin depender de ventanas ni servicios de Wails. Sus operaciones se prueban con directorios temporales y servidores HTTPS de prueba.

`internal/platform` adapta operaciones del sistema. Windows consulta los registros de usuario y sistema para mostrar el entorno efectivo; las modificaciones y su restauración se limitan al registro del usuario. Linux/macOS usan un archivo de entorno y perfiles de shell. El inicio automático usa la clave Run de Windows, LaunchAgents en macOS y autostart `.desktop` en Linux.

Los registros de repositorio e instalación incluyen `tool`. La siguiente herramienta deberá añadir su propia validación y catálogo, manteniendo la infraestructura de configuración, jobs y escritorio compartida. El soporte actual es exclusivamente JDK.

## Persistencia y concurrencia

`state.json` utiliza `schemaVersion: 1`. Las escrituras se realizan en un temporal del mismo directorio, se sincronizan y se renombran. Los errores conservan el estado anterior en memoria y evitan sobrescribir una configuración corrupta al arrancar.

Al crear la configuración se persisten los proveedores incluidos y las preferencias iniciales. Los estados existentes incorporan proveedores ausentes conservando los alias y la activación previa de Adoptium. `preferencesFile` expone la ruta al frontend y su apertura se delega al explorador nativo.

Un mutex protege estado, catálogo consultado y jobs. Las peticiones de catálogo salen del bloqueo durante HTTP. Las instalaciones tienen contexto cancelable y un semáforo de tres operaciones. La configuración del vault se captura al encolar y no puede cambiar mientras haya trabajos activos.

Los jobs son transitorios. Cada paquete se descarga a `.staging-*`, se verifica, se extrae y pasa al directorio final mediante rename. Una instalación gestionada contiene `.tools-switcher-id`. La eliminación comprueba ese marcador y mueve primero a una ubicación de retirada antes de actualizar el inventario.

## Catálogos

`providers.go` define los proveedores incluidos y el adaptador Foojay; `corretto.go` resuelve las descargas oficiales de Amazon. Adoptium, manifiestos y carpetas comparten el modelo `Package`. Los paquetes descubiertos mediante Foojay se resuelven antes de instalar para exigir una URL y un checksum fuerte. El descargador verifica SHA-256 o SHA-512 antes de extraer. Los endpoints se inyectan en las pruebas mediante servidores HTTPS locales.

## Variables

`platform.InspectVariable` separa el valor visible, su origen y los valores de usuario, sistema y sesión. Leer el entorno para mostrarlo no crea una asignación ni cambia la configuración persistente. En Unix se consulta primero el archivo gestionado y después la sesión heredada.

Cada asignación conserva nombre, valor original, existencia previa, instalación asignada y valor aplicado. Activar actualiza JAVA_HOME y PATH como una operación con compensación: si falla un paso, intenta restaurar los anteriores.

Restaurar solo procede cuando el valor actual coincide con el último aplicado por la aplicación. Esta comparación evita perder ediciones realizadas por otras herramientas. La restauración no termina procesos Java ni modifica el entorno de aplicaciones abiertas.

## Actualizaciones

El servicio de escritorio configura el actualizador nativo de Wails con un proveedor de manifiestos HTTPS envuelto para exigir firma. La confianza está anclada en la clave pública incorporada en compilación, no en claves anunciadas por el servidor.

Un temporizador propio respeta las preferencias persistidas. La comprobación automática comienza ocho segundos después del arranque y se repite cada seis horas mientras la aplicación siga abierta. Las nuevas versiones emiten una notificación nativa y se instalan desde Ajustes. El reinicio se bloquea cuando hay instalaciones JDK en curso.

## Validación

- Pruebas Go: catálogos, integridad, persistencia, cancelación, extracción y entorno simulado.
- Pruebas UI: backend simulado, controles React y disposición compacta en Chromium/Edge.
- Prueba nativa opcional Windows: WebView2, bindings reales, validación remota de Adoptium y catálogo real, con configuración temporal.
- CI: comprobación de tipos, compilación, pruebas y validación de cada plataforma en runners nativos.

## Límites conocidos

El descubrimiento busca rutas habituales; no es un inventario exhaustivo del disco. La importación de JDKs externas no transfiere la propiedad de sus archivos al gestor. Los ZIP con enlaces simbólicos y los paquetes MSI no están admitidos. Las descargas canceladas o interrumpidas se vuelven a iniciar completas.

Los paquetes Linux del sistema no se autoactualizan reemplazando `/usr/bin`; se usa el gestor de paquetes. Los artefactos macOS tienen firma ad hoc y no están notarizados. Para añadir arquitecturas publicadas hay que ampliar la matriz y validar cada combinación.
