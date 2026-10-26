# Repositorios de JDKs

## Proveedores incluidos

El selector incluye Eclipse Temurin, Amazon Corretto, Oracle JDK, Oracle OpenJDK, GraalVM Community, Oracle GraalVM, Azul Zulu, BellSoft Liberica, Microsoft OpenJDK y JetBrains Runtime. Temurin se activa inicialmente; los demás se consultan al activarlos. El catálogo filtra por sistema, arquitectura y versión, con ZIP en Windows y TAR.GZ en macOS/Linux. En Linux se solicitan paquetes glibc.

Temurin usa la API oficial de Adoptium. Corretto obtiene los archivos y SHA-256 de los endpoints oficiales de Amazon; fija la URL de la versión y comprueba que no haya cambiado durante la consulta del checksum. Los demás usan [Foojay Disco API](https://github.com/foojayio/discoapi) para descubrir versiones y resolver las descargas publicadas por sus proveedores. Si no existe un hash fuerte en esa respuesta, puede utilizarse el digest SHA-256/SHA-512 del mismo archivo publicado en una release de GitHub. Un paquete sin checksum fuerte se rechaza.

Activar o sincronizar valida un paquete de muestra mediante una petición acotada. El número mostrado corresponde a la última consulta, no a todas las versiones de la distribución. Una versión publicada puede carecer de paquete para determinada plataforma; en ese caso su catálogo queda vacío. La selección entre versiones se conserva, pero cambiar de proveedor inicia otra selección.

Las condiciones de uso y licencias dependen de cada distribución. El selector no omite autenticaciones ni acepta licencias en nombre del usuario. Solo admite descargas públicas disponibles mediante esos catálogos.

## Tipos admitidos

- Adoptium: URLs HTTPS del host `api.adoptium.net`, normalizadas a `https://api.adoptium.net/v3`.
- Manifiesto propio: una URL HTTPS que devuelve el formato siguiente.
- Carpeta local: instalaciones y paquetes ZIP/TAR.GZ. Los archivos comprimidos se leen en el primer nivel; las carpetas se inspeccionan hasta seis niveles.

## Manifiesto propio

```json
{
  "schemaVersion": 1,
  "packages": [
    {
      "name": "Mi OpenJDK 21",
      "version": "21.0.9",
      "vendor": "Mi proveedor",
      "os": "windows",
      "arch": "amd64",
      "url": "https://downloads.example.com/openjdk-21-windows-x64.zip",
      "sha256": "REEMPLAZAR_POR_64_CARACTERES_HEXADECIMALES",
      "filename": "openjdk-21-windows-x64.zip",
      "size": 190000000
    }
  ]
}
```

También se admite el campo `sha512` con 128 caracteres hexadecimales en lugar de `sha256`. El checksum del ejemplo es un marcador: el validador lo rechazará hasta sustituirlo por un SHA-256 real. Puedes calcularlo en PowerShell con `(Get-FileHash archivo.zip -Algorithm SHA256).Hash.ToLowerInvariant()` o con `sha256sum` en Linux.

`os`: `windows`, `darwin` o `linux`. `arch`: `amd64` o `arm64`, según la compilación. `filename` debe ser un nombre simple terminado en `.zip` o `.tar.gz`. La URL debe ser HTTPS y no puede incluir credenciales. No se admiten mecanismos de autenticación propios ni scraping de páginas HTML.

Los identificadores internos se calculan a partir del repositorio y la URL; el manifiesto no necesita proporcionarlos. `localPath` del manifiesto remoto se ignora. Solo aparecen los paquetes compatibles con el sistema y arquitectura actuales. En un manifiesto propio el filtro de versión de Adoptium no aplica: el catálogo muestra todos sus paquetes compatibles.

El servidor debe responder un JSON de hasta 8 MiB. Se valida el esquema, los campos de todos los paquetes, los checksums y el acceso a un enlace compatible de muestra. La comprobación de muestra lee una respuesta acotada; cada descarga completa vuelve a verificar su integridad.

Los repositorios desactivados no permiten iniciar instalaciones. Las instalaciones ya en curso continúan aunque se desactive o quite su origen. El inventario conserva el identificador del repositorio original.

## Integridad y extracción

La firma de una actualización de tools-switcher y el checksum de una JDK son mecanismos diferentes. Los checksums de JDKs se obtienen del proveedor o del manifiesto, que debe estar servido por una fuente de confianza.

Los paquetes se extraen a un directorio nuevo. Se rechazan rutas absolutas, escapes `..`, nombres con separadores Windows y tipos especiales no admitidos. Los enlaces TAR solo se aceptan si sus destinos resuelven dentro de la extracción. Los enlaces simbólicos ZIP se rechazan.

Una instalación requiere `release`, `JAVA_VERSION`, `bin/java` y `bin/javac` (con `.exe` en Windows). Las carpetas locales se importan mediante referencia y no se ejecutan para inspeccionarlas.
