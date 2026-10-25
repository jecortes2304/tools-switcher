# tools-switcher

Aplicación de escritorio compacta para gestionar herramientas de desarrollo. La primera herramienta disponible es Java: permite consultar catálogos, instalar JDKs, importar instalaciones existentes y configurar el entorno del usuario.

Construida con Go, Wails 3 y React con TypeScript. El identificador de aplicación es `com.cortestudios.tools-switcher`.

## Funcionalidades

- Inventario de JDKs con búsqueda, proveedor, repositorio y ubicación.
- Activación de una JDK mediante `JAVA_HOME` y `PATH`.
- Asignación de una JDK a variables existentes o nuevas y restauración de los valores anteriores.
- Catálogo de Eclipse Temurin mediante la API oficial de Adoptium.
- Repositorios propios con manifiesto JSON sobre HTTPS.
- Carpetas offline con JDKs descomprimidas y archivos ZIP o TAR.GZ compatibles.
- Selección de varias versiones, tres instalaciones concurrentes, progreso y cancelación.
- Validación SHA-256 de descargas remotas y extracción limitada al destino.
- Importación manual y detección en ubicaciones habituales.
- Temas oscuro, claro y del sistema.
- Carpeta vault configurable, inicio con la sesión y cierre a la bandeja.
- Comprobación periódica, notificación nativa y actualizaciones firmadas de la aplicación.

Las actualizaciones se habilitan en las compilaciones que incorporan la clave pública de publicación. El desarrollo local funciona sin claves ni certificados.

## Requisitos para desarrollar

- Go 1.25 o posterior compatible con `go.mod`.
- Node.js 24 y npm.
- Wails CLI **v3.0.0-beta.16**, igual que el módulo Go.
- Windows: WebView2 Runtime y herramientas de compilación indicadas por `wails3 doctor`.
- macOS: Xcode Command Line Tools.
- Linux: compilador C, pkg-config, GTK4 y WebKitGTK 6.0; en Ubuntu 24.04: `build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`.

```sh
git clone https://github.com/jecortes2304/tools-switcher.git
cd tools-switcher
git switch develop
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.16
go mod download
npm --prefix frontend ci
wails3 dev
```

`wails3 dev` genera los bindings, arranca Vite y abre la ventana nativa con recarga durante el desarrollo. El puerto local predeterminado es 9245.

Para compilar sin ejecutar:

```sh
wails3 build
```

El resultado se escribe en `bin/`. Para generar los mismos artefactos versionados que CI:

```sh
go run ./cmd/ci build
```

En local produce el binario de Windows, el bundle comprimido de macOS o el binario comprimido de Linux, según el sistema actual. El empaquetado de producción se ejecuta desde la pipeline de `main`.

## Uso

### 1. Añadir un repositorio

En **Repositorios**, selecciona URL remota y añade `https://api.adoptium.net/v3`. Puedes ponerle un alias. tools-switcher consulta el catálogo y comprueba un enlace de muestra antes de guardarlo.

Una página web genérica no es un catálogo. Las fuentes remotas admitidas son la API de Adoptium y el [manifiesto propio](docs/repositories.md). La validación previa no sustituye la comprobación de integridad al descargar.

Para trabajar offline, selecciona una carpeta con JDKs o paquetes `.zip` / `.tar.gz` en su primer nivel. Las carpetas de JDK se buscan hasta seis niveles. Se filtran los archivos comprimidos según sistema y arquitectura declarados en `release`.

Desactivar un repositorio impide nuevas instalaciones desde ese origen. Eliminarlo conserva las JDKs ya instaladas.

### 2. Instalar o importar una JDK

En **JDKs → Instalar**, elige repositorio y versión. Marca paquetes y cambia de versión para añadir más a la selección. **Instalar selección** los añade a la cola, con un máximo de tres operaciones simultáneas.

Las descargas remotas deben incluir un SHA-256 válido. El paquete se descarga a una carpeta temporal dentro del vault, se verifica y se extrae. Solo se registra cuando contiene exactamente una JDK con `release`, `bin/java` y `bin/javac`. La cancelación o un error limpia el directorio temporal.

**Importar** registra la carpeta raíz de una JDK existente. **Detectar** consulta `JAVA_HOME`, el vault y ubicaciones habituales de Java de cada plataforma; no recorre todos los discos. Las carpetas offline descomprimidas también se registran como instalaciones externas.

### 3. Activar y configurar variables

**Activar** configura `JAVA_HOME` y añade el directorio `bin` a `PATH`. Los cambios del par se revierten si una operación falla.

El menú `⋮` y el clic derecho permiten establecer solo `JAVA_HOME`, gestionar PATH, asignar otra variable, abrir la ubicación, restaurar valores anteriores y retirar la JDK.

En Windows se modifica el entorno del usuario en `HKCU\Environment` y se anuncia el cambio al sistema. El PATH del sistema puede resolver otro Java antes que el PATH del usuario; **Ajustes → Diagnóstico** muestra la resolución prevista. Los procesos ya abiertos mantienen su entorno: abre una terminal nueva.

En Linux y macOS se genera `environment.sh` en el directorio de configuración y se carga desde los perfiles de Bash y, en macOS, Zsh. Se aplica a sesiones nuevas de esos shells; no cambia el entorno de procesos ya abiertos ni el de todas las aplicaciones gráficas. Fish, PowerShell y otros shells requieren cargar un equivalente manualmente.

La aplicación guarda el valor original de cada variable que gestiona. Si detecta una edición externa posterior, evita sobrescribirla: debes volver al valor gestionado o ajustar la variable manualmente antes de restaurarla desde la aplicación.

### 4. Retirar instalaciones

Las JDKs instaladas por tools-switcher pueden borrarse del vault. Se comprueba su marcador de propiedad y se restauran sus variables antes de retirarlas. Si están en uso, Windows puede impedir la operación.

Las instalaciones externas se **desvinculan** y conservan sus archivos.

### 5. Preferencias y bandeja

En **Ajustes** puedes elegir tema y vault, iniciar con la sesión, mantener la aplicación en la bandeja al cerrar y habilitar comprobaciones de actualización.

Cambiar de vault afecta a instalaciones nuevas. Para salir completamente usa **Salir** en el menú de la bandeja. La integración de bandeja en Linux depende del escritorio y sus extensiones.

## Datos locales

Se usa `os.UserConfigDir()/tools-switcher/state.json`: `%APPDATA%\tools-switcher` en Windows, `~/Library/Application Support/tools-switcher` en macOS y `$XDG_CONFIG_HOME/tools-switcher` o `~/.config/tools-switcher` en Linux.

El vault predeterminado es `~/.tools-switcher/vault`. La configuración incluye esquema versionado, preferencias, repositorios, instalaciones y copias de los valores de entorno modificados. Un archivo corrupto produce un error y se conserva para poder recuperarlo.

Las colas viven en memoria; las descargas se cancelan al salir. No se reanudan tras reiniciar. Se admiten hasta 50 paquetes por lote, 8 GiB por paquete/extracción y 30 minutos por instalación.

`TOOLS_SWITCHER_CONFIG_DIR` permite usar un directorio de configuración absoluto alternativo para pruebas. `--background` inicia la ventana oculta.

## Estructura

```text
main.go                     Composición de la aplicación, ventana y bandeja
main_windows_test.go        Prueba nativa opcional en Windows
internal/manager/           Catálogos, paquetes, inventario, descargas y persistencia
internal/platform/          Variables, apertura de carpetas e inicio con sesión
internal/desktop/           Servicios Wails, notificaciones y actualizador
frontend/src/app/           Acceso tipado a los servicios
frontend/src/components/    Controles y diálogos compartidos
frontend/src/features/      JDKs, repositorios, catálogo y ajustes
frontend/src/styles.css     Tokens y estilos de los temas
frontend/bindings/          Código generado por Wails
frontend/tests/             Pruebas de interfaz
cmd/ci/                     Versionado, compilación, firmas y manifiestos de release
build/                      Recursos y tareas de empaquetado por plataforma
docs/                       Repositorios, arquitectura y publicación
.github/workflows/          Validación, artefactos y releases
```

Repositorios e instalaciones tienen un campo `tool`; actualmente su valor es `jdk`. Otra herramienta podrá añadir su catálogo y operaciones sin cambiar la persistencia y el shell de navegación completo. La detección y validación de instalaciones Java permanecen dentro del gestor de JDKs.

Los bindings son generados, no se editan a mano:

```sh
wails3 generate bindings -ts -i
```

El código propio evita comentarios explicativos. Las directivas de compilación, los encabezados de archivos generados y los comentarios de las plantillas de Wails son gestionados por sus herramientas.

## Comprobaciones

```sh
wails3 generate bindings -ts -i
npm --prefix frontend run build
go test ./...
go vet ./...
cd frontend
npx playwright install chromium
npm run test:ui
```

Las pruebas Go usan directorios temporales y un entorno simulado: no modifican tus variables reales. Cubren checksums, repositorios inválidos, extracción insegura, cancelación, persistencia, activación y restauración.

Las pruebas UI usan un backend simulado y comprueban navegación, búsqueda, selección entre versiones, ajustes, confirmación de eliminación y ventana compacta. Generan capturas en `artifacts/ui/`. Con Edge instalado puedes usar `PLAYWRIGHT_CHANNEL=msedge` en vez de descargar Chromium.

En Windows, después de compilar, `$env:TOOLS_SWITCHER_NATIVE_TEST="1"; go test -run TestNativeSmoke -count=1 -timeout 90s .` comprueba el puente real de Wails y el catálogo público de Adoptium con configuración aislada. Requiere internet y WebView2; no activa JDKs ni modifica PATH.

## Ramas y distribución

- `feature/*` y PRs: validación y ZIP del código con retención de 15 días.
- `develop`: binarios Windows x64, macOS ARM64 y Linux x64 como artefactos de Actions durante 15 días.
- `main`: validación, binarios, instalador de Windows, DMG, AppImage, DEB, RPM, checksums y manifiesto firmado; publica una GitHub Release cuando todos los trabajos terminan correctamente.

La versión de publicación se obtiene de `VERSION`. Incrementa este archivo antes de cada release. No se publican dos versiones con el mismo tag.

La configuración de claves y certificado está explicada en [Publicación y credenciales](docs/releases.md). macOS se distribuye con firma ad hoc y sin notarización; no requiere Apple Developer y Gatekeeper puede solicitar una autorización manual.

## Alcance actual

El proveedor integrado es Temurin; otros proveedores se pueden exponer con un manifiesto propio o incorporar mediante adaptadores. El catálogo usa paquetes portables ZIP/TAR.GZ, no instaladores MSI de JDKs. Las actualizaciones de tools-switcher reemplazan el ejecutable o bundle de usuario; las instalaciones Linux gestionadas por DEB/RPM deben actualizarse mediante su gestor de paquetes.
