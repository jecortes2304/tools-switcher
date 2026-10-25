# Publicación y credenciales

## Pipelines

`features.yml` valida ramas y pull requests y conserva un ZIP del código exacto durante 15 días. `desktop.yml` construye en runners nativos de Windows, macOS y Linux al recibir cambios en `develop` o `main`.

La matriz inicial es Windows amd64, macOS arm64 y Linux amd64. El núcleo admite metadatos de ambas arquitecturas, pero no se publican otras combinaciones sin añadir su runner y validarlas.

`develop` produce artefactos sin certificados. `main` exige claves de actualizaciones y certificado Windows, construye instaladores y publica la release solo después de verificar los artefactos de todas las plataformas.

La versión se lee de `VERSION` y se aplica al binario y sus metadatos durante la compilación. El tag es `vX.Y.Z`. Antes de promover otra release, incrementa `VERSION` en una feature y lleva el cambio a `develop` y después a `main`.

## Configuración en GitHub

Abre el repositorio → **Settings → Secrets and variables → Actions**.

| Nombre | Tipo | Origen |
| --- | --- | --- |
| `UPDATER_PUBLIC_KEY_BASE64` | Variable de Actions | Clave pública generada con Wails y codificada en Base64 |
| `UPDATER_PRIVATE_KEY_BASE64` | Secret de Actions | Clave privada generada con Wails y codificada en Base64 |
| `WINDOWS_CERTIFICATE_BASE64` | Secret de Actions | Certificado Authenticode exportable en formato PFX, codificado en Base64 |
| `WINDOWS_CERTIFICATE_PASSWORD` | Secret de Actions | Contraseña de ese PFX |

La prevalidación de `main` comprueba que las claves Ed25519 son válidas y forman pareja. El certificado se usa solo en el runner Windows. La clave privada de actualizaciones se usa para la prevalidación y el manifiesto final; nunca se incorpora al programa. La clave pública se incorpora al ejecutable con `-ldflags`.

### Crear las claves del actualizador

Con la misma CLI de desarrollo:

```sh
wails3 updater genkey
```

Se crean `updater.key` y `updater.key.pub`. Ejecuta el comando en una ubicación segura. Haz copia de seguridad de la clave privada: cambiarla requiere planificar la migración de las aplicaciones instaladas.

En PowerShell, copia cada valor al campo correspondiente de GitHub:

```powershell
[Convert]::ToBase64String([IO.File]::ReadAllBytes((Resolve-Path ./updater.key.pub))) | Set-Clipboard
[Convert]::ToBase64String([IO.File]::ReadAllBytes((Resolve-Path ./updater.key))) | Set-Clipboard
```

Ejecuta y pega uno por uno. No pegues las claves en issues, PRs o el chat. `.gitignore` excluye los nombres de clave predeterminados y los PFX/P12, pero guarda estos archivos fuera del repositorio.

### Certificado de Windows

Si ya tienes un certificado exportable de firma de código, abre **Administrar certificados de usuario → Personal → Certificados**, selecciona el de firma de código y usa **Todas las tareas → Exportar → Exportar la clave privada → PFX**. El asistente permite protegerlo con contraseña. Un certificado cuya clave esté en hardware o un servicio remoto puede no permitir exportación: en ese caso hay que adaptar el paso de firma a ese proveedor.

Codifica el PFX:

```powershell
[Convert]::ToBase64String([IO.File]::ReadAllBytes((Resolve-Path ./certificado.pfx))) | Set-Clipboard
```

La contraseña va en su secret separado. El ejecutable y el instalador se firman con timestamp. La firma no garantiza que SmartScreen haya adquirido reputación para una aplicación nueva.

No hace falta ningún PAT: la release usa el `GITHUB_TOKEN` automático con `contents: write` únicamente en el trabajo de publicación. GitHub Actions debe estar habilitado en el repositorio.

## macOS sin Apple Developer

Se crea una aplicación `.app`, se firma ad hoc y se comprime para actualizaciones. También se produce un `.dmg` para instalación. No se solicita una cuenta Apple, certificado Developer ID ni notarización. Gatekeeper puede bloquear la apertura inicial o requerir que el usuario la autorice en Privacidad y seguridad. La firma criptográfica del actualizador sigue verificando sus descargas, pero no sustituye la notarización de Apple.

## Linux

Se publican un binario portable TAR.GZ, AppImage, DEB y RPM. Requieren el stack GTK4/WebKitGTK 6.0 correspondiente de la distribución. Las dependencias de DEB/RPM están en `build/linux/nfpm/nfpm.yaml`.

La autoactualización se destina a ejecutables o bundles instalados en una ubicación de usuario escribible. Una instalación bajo `/usr/bin` o mediante DEB/RPM se actualiza con su gestor de paquetes, sin elevar privilegios desde la aplicación. El AppImage se gestiona como artefacto de distribución separado del binario portable; su actualización debe hacerse reemplazando el AppImage completo.

## Publicación e integridad

La pipeline genera artefactos para actualizar separados de los instaladores:

- Windows: ejecutable firmado.
- macOS: ZIP con un único bundle `.app` firmado ad hoc.
- Linux: TAR.GZ con un único ejecutable.

`wails3 updater manifest` calcula hashes y firma cada artefacto. `wails3 updater verify` verifica los archivos descargados de los jobs antes de publicar el manifiesto. La aplicación rechaza releases sin firma y verifica la descarga con la clave pública incorporada.

Se publica `SHA256SUMS` para todos los paquetes. El manifiesto estable se sirve en `https://github.com/jecortes2304/tools-switcher/releases/latest/download/manifest.json`. Primero se crea una release como borrador, se cargan los archivos y al final se publica; así se evita anunciar una actualización incompleta.

Si falla una publicación después de crear el borrador, revisa y elimina solo ese borrador incompleto antes de reejecutar el job. No borres ni reutilices una release ya publicada; usa una versión nueva.

## Pendiente antes de la primera release

1. Configurar los cuatro valores de la tabla.
2. Revisar `docs/release-notes.md`.
3. Probar las builds de `develop` en los sistemas de destino, especialmente macOS y Linux.
4. Fusionar la versión elegida en `main` para iniciar la publicación.

Las features y builds de desarrollo no necesitan esos secretos.
