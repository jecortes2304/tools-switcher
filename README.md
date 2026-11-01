# tools-switcher

Aplicación de escritorio compacta para gestionar herramientas de desarrollo. La primera herramienta disponible es Java: permite consultar catálogos, instalar JDKs, importar instalaciones existentes y configurar el entorno del usuario.

Construida con Go, Wails 3 y React con TypeScript. El identificador de aplicación es `com.cortestudios.tools-switcher`.

## Funcionalidades

- Inventario de JDKs con búsqueda, proveedor, repositorio y ubicación.
- Activación de una JDK mediante `JAVA_HOME` y `PATH`.
- Asignación de una JDK a variables existentes o nuevas y restauración de los valores anteriores.
- Diez proveedores incluidos: Temurin, Corretto, Oracle JDK, Oracle OpenJDK, GraalVM Community, Oracle GraalVM, Zulu, Liberica, Microsoft y JetBrains Runtime.
- Repositorios propios con manifiesto JSON sobre HTTPS.
- Carpetas offline con JDKs descomprimidas y archivos ZIP o TAR.GZ compatibles.
- Selección de varias versiones, tres instalaciones concurrentes, progreso y cancelación.
- Validación SHA-256 o SHA-512 de descargas remotas y extracción limitada al destino.
- Importación manual y detección en ubicaciones habituales.
- Temas oscuro, claro y del sistema.
- Carpeta vault configurable, inicio con la sesión y cierre a la bandeja.
- Comprobación periódica, notificación nativa y actualizaciones firmadas de la aplicación.
- Asistente de actualización con novedades, porcentaje de descarga, velocidad, reintento y reinicio aplazable.

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

En **Repositorios**, busca y selecciona un proveedor incluido y pulsa **Activar**. Temurin viene activado al crear la configuración. La activación consulta versiones, comprueba un paquete compatible y valida el acceso a una descarga con checksum. Puedes sincronizar cada fuente o desactivarla conservando las JDKs instaladas.

Para probar Oracle, selecciona **Oracle JDK**, actívalo y abre **JDKs → Instalar → Oracle JDK**. Elige una versión disponible y marca el paquete. Oracle OpenJDK y las dos distribuciones de GraalVM aparecen como proveedores separados. HotSpot es una máquina virtual usada por varias distribuciones; la distribución de Amazon se llama Corretto.

**Añadir carpeta o catálogo personalizado** permite registrar una carpeta offline, la API `https://api.adoptium.net/v3` o un [manifiesto propio](docs/repositories.md). Una página web de descargas o un enlace ZIP aislado no constituyen un catálogo compatible.

Para trabajar offline, selecciona una carpeta con JDKs o paquetes `.zip` / `.tar.gz` en su primer nivel. Las carpetas de JDK se buscan hasta seis niveles. Se filtran los archivos comprimidos según sistema y arquitectura declarados en `release`.

Los proveedores incluidos se desactivan; los catálogos personalizados también pueden eliminarse. Ambas operaciones conservan las instalaciones existentes. Consulta [cómo se obtienen y validan los paquetes](docs/repositories.md).

### 2. Instalar o importar una JDK

En **JDKs → Instalar**, elige repositorio y versión. Marca paquetes y cambia de versión para añadir más a la selección. **Instalar selección** los añade a la cola, con un máximo de tres operaciones simultáneas.

Las descargas remotas deben incluir un SHA-256 o SHA-512 válido. El paquete se descarga a una carpeta temporal dentro del vault, se verifica y se extrae. Solo se registra cuando contiene exactamente una JDK con `release`, `bin/java` y `bin/javac`. La cancelación o un error limpia el directorio temporal.

**Importar** registra la carpeta raíz de una JDK existente. **Detectar** consulta `JAVA_HOME`, el vault y ubicaciones habituales de Java de cada plataforma; no recorre todos los discos. Las carpetas offline descomprimidas también se registran como instalaciones externas.

### 3. Activar y configurar variables

**Activar** configura `JAVA_HOME` y añade el directorio `bin` a `PATH`. Los cambios del par se revierten si una operación falla.

El menú `⋮` y el clic derecho permiten gestionar PATH y abrir **Variables de entorno**: incluye accesos a `JAVA_HOME`, `JDK_HOME`, `GRAALVM_HOME` y `JRE_HOME`, además de una variable personalizada. `GRAALVM_HOME` se habilita para instalaciones GraalVM. Son convenciones utilizadas por herramientas distintas, no una lista exhaustiva ni variables obligatorias. El menú también permite abrir la ubicación, restaurar valores anteriores y retirar la JDK.

Al arrancar se consulta `JAVA_HOME` aunque todavía no haya instalaciones registradas. En Windows se leen los ámbitos de usuario y sistema; en su ausencia se usa el valor heredado de la sesión. La cabecera muestra su origen y **Ajustes** permite comparar los valores. **Detectar** incorpora la instalación al inventario si su carpeta contiene una JDK válida.

En Windows se modifica el entorno del usuario en `HKCU\Environment` y se anuncia el cambio al sistema. El PATH del sistema puede resolver otro Java antes que el PATH del usuario; **Ajustes → Diagnóstico** muestra la resolución prevista. Los procesos ya abiertos mantienen su entorno: abre una terminal nueva.

En Linux y macOS se genera `environment.sh` en el directorio de configuración y se carga desde los perfiles de Bash y, en macOS, Zsh. Se aplica a sesiones nuevas de esos shells; no cambia el entorno de procesos ya abiertos ni el de todas las aplicaciones gráficas. Fish, PowerShell y otros shells requieren cargar un equivalente manualmente.

La aplicación guarda el valor original de cada variable que gestiona. Si detecta una edición externa posterior, evita sobrescribirla: debes volver al valor gestionado o ajustar la variable manualmente antes de restaurarla desde la aplicación.

### 4. Retirar instalaciones

Las JDKs instaladas por tools-switcher pueden borrarse del vault. Se comprueba su marcador de propiedad y se restauran sus variables antes de retirarlas. Si están en uso, Windows puede impedir la operación.

Las instalaciones externas se **desvinculan** y conservan sus archivos.

### 5. Preferencias y bandeja

En **Ajustes** puedes elegir tema y vault, iniciar con la sesión, mantener la aplicación en la bandeja al cerrar y habilitar comprobaciones de actualización. **Archivo de preferencias** muestra la ruta de `state.json`, permite copiarla y ofrece **Mostrar archivo de preferencias** para localizarlo en el explorador.

Cambiar de vault afecta a instalaciones nuevas. Para salir completamente usa **Salir** en el menú de la bandeja. La integración de bandeja en Linux depende del escritorio y sus extensiones.

## Datos locales

Se usa `os.UserConfigDir()/tools-switcher/state.json`: `%APPDATA%\tools-switcher` en Windows, `~/Library/Application Support/tools-switcher` en macOS y `$XDG_CONFIG_HOME/tools-switcher` o `~/.config/tools-switcher` en Linux.

El vault predeterminado es `~/.tools-switcher/vault`. La configuración incluye esquema versionado, preferencias, repositorios, instalaciones y copias de los valores de entorno modificados. Un archivo corrupto produce un error y se conserva para poder recuperarlo.

Para guardar una copia o recuperar otra configuración, localiza el archivo desde Ajustes, sal completamente mediante **Salir** en la bandeja y copia o reemplaza `state.json`. Conserva una copia antes de reemplazarlo. Al borrarlo se recrean las preferencias y los proveedores iniciales; esto no borra las JDKs del disco ni restablece las variables del sistema. También se pierden los valores originales guardados para restaurar variables. Usa **Detectar** para registrar de nuevo las instalaciones existentes.

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
node --test scripts/release.test.mjs
node scripts/release.mjs version
cd frontend
npx playwright install chromium
npm run test:ui
```

Las pruebas Go usan directorios temporales y un entorno simulado: no modifican tus variables reales. Cubren checksums, repositorios inválidos, extracción insegura, cancelación, persistencia, activación y restauración.

Las pruebas UI usan un backend simulado y comprueban navegación, búsqueda, selección entre versiones, ajustes, confirmación de eliminación y ventana compacta. Generan capturas en `artifacts/ui/`. Con Edge instalado puedes usar `PLAYWRIGHT_CHANNEL=msedge` en vez de descargar Chromium.

En Windows, después de compilar, `$env:TOOLS_SWITCHER_NATIVE_TEST="1"; go test -run TestNativeSmoke -count=1 -timeout 90s .` comprueba el puente real de Wails y el catálogo público de Adoptium con configuración aislada. Requiere internet y WebView2; no activa JDKs ni modifica PATH.

## Pipeline: desde una feature hasta la actualización instalada

### Preparación inicial del repositorio

Las ramas permanentes son **develop** y **main**. Los workflows usan `GITHUB_TOKEN`; no necesitan un PAT ni nuevas claves de firma para el versionado.

En **Settings → Actions → General → Workflow permissions**, activa **Allow GitHub Actions to create and approve pull requests**. GitHub agrupa la creación y aprobación en esa opción; nuestros workflows solo crean PRs y no los aprueban ni fusionan automáticamente. Los permisos de escritura se conceden únicamente a los jobs que preparan la release o la publican.

Conserva las claves y el certificado indicados en [Publicación y credenciales](docs/releases.md). El repositorio o el alojamiento del manifiesto y de los archivos de actualización debe ser accesible para la app sin iniciar sesión. No cambies la clave del actualizador entre versiones.

### 1. Crear una feature

```sh
git fetch origin --prune
git switch develop
git pull --ff-only origin develop
git switch -c feature/mi-cambio
```

Haz los cambios, ejecuta las comprobaciones y crea commits y un PR con un título convencional, por ejemplo `feat(updater): añadir asistente de actualización`. **No edites VERSION manualmente.**

| Título o commit | Resultado previsto |
| --- | --- |
| `fix: corregir un error` | Incrementa patch: 0.1.0 → 0.1.1 |
| `feat: añadir una funcionalidad` | Incrementa minor: 0.1.0 → 0.2.0 |
| `docs: mejorar documentación` | No publica una versión por sí solo |
| `chore: mantener herramientas` | No publica una versión por sí solo |
| `feat!: cambio incompatible` o pie `BREAKING CHANGE:` | Antes de 1.0 incrementa minor; desde 1.0 incrementa major |

La versión refleja el conjunto de cambios desde la última publicación: varios fixes y una feature producen una sola versión minor. El nombre de la rama no decide el incremento. Los cambios técnicos que deban publicarse deben describirse con el tipo correspondiente a su efecto, por ejemplo `fix(ci): corregir generación de instaladores`.

### 2. Validar e integrar en develop

`Feature validation` comprueba el título del PR, la coherencia de versiones, formato, compilación y pruebas en Windows, macOS y Linux. También comprueba la firma Windows con un certificado temporal y la generación del manifiesto con claves de prueba. Cada push a una rama de trabajo conserva un ZIP del código durante 15 días. Las validaciones completas se ejecutan en el PR o por solicitud del bot, evitando repetir la matriz por push y PR.

Para features usa **Squash and merge** conservando el título convencional como mensaje del commit final. Los títulos convencionales también se validan al editar el PR. Un PR hacia `main` debe proceder de `develop`.

El merge en **develop** inicia dos procesos independientes:

- **Desktop builds and releases** genera los binarios de desarrollo para Windows x64, macOS ARM64 y Linux x64. Se descargan desde **Actions → ejecución → Artifacts** y se conservan 15 días. No se publica una release estable ni se crean instaladores de producción desde develop.
- **Prepare release** analiza los commits y mantiene un PR del bot hacia develop, por ejemplo **chore(develop): release 0.2.0**. Actualiza `VERSION`, `.release-please-manifest.json`, los metadatos del frontend y `CHANGELOG.md`. Mientras esté abierto se irá actualizando con nuevas features y fixes.

Los PRs creados con `GITHUB_TOKEN` no disparan normalmente otro workflow por sí solos. Por eso el bot solicita explícitamente `Feature validation` con `workflow_dispatch` sobre su rama; no hace falta un token personal. El título de ese PR sigue siendo convencional.

### 3. Aprobar la versión que saldrá

Abre el PR de release del bot. Revisa el número propuesto, las notas y sus comprobaciones. Puedes seguir integrando features antes de fusionarlo: el bot recalculará la propuesta.

Cuando esté listo, **fusiona el PR de release en develop**. Ese merge prepara el candidato; todavía no publica. Desde ese momento promueve ese candidato a main antes de integrar nuevas features en develop. El bot no prepara otra versión mientras esta siga pendiente de publicación.

### 4. Promover develop a main

Crea un PR con **base main** y **compare develop**, revisa las comprobaciones y fusiónalo con **Create a merge commit**. Evita squash/rebase para promociones y sincronizaciones entre las ramas permanentes: necesitamos conservar los commits y los tags de las releases en el historial.

La pipeline de main verifica que los números coincidan, que exista un PR de release revisado, que el código de la aplicación corresponda a ese candidato y que el tag no exista ya. Se permiten correcciones de pipeline, scripts de publicación y documentación posteriores al candidato para poder recuperar una publicación fallida. Así se evita publicar una versión sin preparar o incorporar cambios posteriores sin documentar.

### 5. Construir, firmar y publicar

`Desktop builds and releases` ejecuta:

1. **prepare:** valida candidato, versión y credenciales.
2. **build:** compila en paralelo sobre runners nativos de Windows, macOS y Linux, ejecuta las pruebas y genera los artefactos de cada sistema.
3. **release:** reúne esos archivos, genera el manifiesto con las notas de esa versión en `CHANGELOG.md`, firma y verifica los artefactos de actualización y calcula `SHA256SUMS`. Crea el tag `vX.Y.Z` y una GitHub Release en borrador, carga todos los archivos y finalmente la publica.
4. **sync-release:** marca el PR del bot como publicado y sincroniza develop con el commit publicado de main mediante avance rápido, sin forzar el historial. Si ya hay commits nuevos en develop o la protección de ramas impide hacerlo, abre un PR main → develop para que lo revises y fusiones antes de preparar otra versión.

Cada job usa una máquina nueva. La preparación de herramientas y los paquetes de Linux pueden tardar varios minutos aunque la compilación Go sea rápida. El job release espera a las tres plataformas. La publicación estable solo aparece cuando todos los artefactos están listos.

| Plataforma | Descarga para actualizar | Instalación inicial |
| --- | --- | --- |
| Windows x64 | EXE firmado | Setup EXE |
| macOS ARM64 | ZIP con app | DMG |
| Linux x64 | TAR.GZ portable | AppImage, DEB y RPM |

macOS utiliza firma ad hoc sin notarización. El certificado Windows autofirmado sirve para pruebas, pero no elimina SmartScreen. Linux instalado por DEB/RPM se actualiza con el gestor de paquetes; el AppImage se sustituye completo.

### 6. Probar desde una versión instalada

Conserva instalada la versión anterior y las mismas claves de actualización. En la app activa **Ajustes → Buscar actualizaciones** y permite las notificaciones del sistema. Debe estar abierta o en la bandeja.

La búsqueda automática ocurre ocho segundos después del arranque y cada seis horas. Para probar inmediatamente pulsa **Ajustes → Abrir asistente de actualización → Buscar actualizaciones**. Cuando encuentre una versión superior, el asistente muestra el salto de versión y las novedades. **Descargar e instalar** inicia la descarga, muestra porcentaje y velocidad cuando el servidor comunica el tamaño, verifica la firma y prepara la sustitución. Puedes ocultar el asistente sin interrumpir el proceso. Si falla, muestra el error y permite reintentar. **Reiniciar ahora** aplica la actualización y vuelve a abrir la app; **Reiniciar más tarde** conserva el estado preparado durante esa sesión.

El reinicio y la preparación se bloquean mientras haya instalaciones JDK activas. El actualizador conserva preferencias e inventario. Una firma inválida impide aplicar la descarga.

Para la primera integración de esta automatización, usa **Create a merge commit** en el PR de la feature: también incorpora al historial de develop el commit publicado de main. Después, las features normales pueden usar squash.

Para probar este cambio desde **0.1.0**, integra primero la feature en develop, espera el PR automático de **0.2.0**, fusiónalo y promueve develop a main. La 0.1.0 descargará esa primera actualización con sus controles antiguos de Ajustes; después de reiniciar ya tendrá el asistente nuevo. Para ver una notificación inmediata puedes salir completamente desde la bandeja y abrir de nuevo tras publicar la release.

### Recuperación de una pipeline fallida

- **Falla una feature:** corrige su rama; el PR se vuelve a validar.
- **Falla un build de main:** no se publica la release; consulta el job que falló. Los artefactos de otros jobs pueden existir en Actions.
- **Falla release antes de publicar:** revisa el fallo y cualquier borrador incompleto. No elimines ni reutilices versiones ya publicadas.
- **Cambias el workflow para corregirlo:** integra la corrección y ejecuta el commit nuevo. Las correcciones de CI y documentación no requieren preparar otra versión antes de la primera publicación de ese candidato. Reejecutar un job antiguo conserva su workflow antiguo. Si necesitas cambiar el código de la app después de aprobar el candidato y antes de publicarlo, revierte primero el PR de versión en develop y retira su etiqueta `autorelease: pending`; integra la corrección y espera la propuesta nueva del bot.
- **Falla Prepare release por permisos:** revisa la opción de creación de PRs y vuelve a ejecutar ese workflow sobre develop.
- **Aparece un PR de sincronización main → develop:** fusiónalo con merge commit antes de preparar otra release. El bot no hace force-push ni descarta commits nuevos.

## Alcance actual

Hay diez proveedores incluidos; otros se pueden exponer con un manifiesto propio o incorporar mediante adaptadores. El catálogo usa paquetes portables ZIP/TAR.GZ, no instaladores MSI de JDKs. Las actualizaciones de tools-switcher reemplazan el ejecutable o bundle de usuario; las instalaciones Linux gestionadas por DEB/RPM deben actualizarse mediante su gestor de paquetes.
