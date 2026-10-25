# Repositorios de JDKs

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

El checksum del ejemplo es un marcador: el validador lo rechazará hasta sustituirlo por un SHA-256 real. Puedes calcularlo en PowerShell con `(Get-FileHash archivo.zip -Algorithm SHA256).Hash.ToLowerInvariant()` o con `sha256sum` en Linux.

`os`: `windows`, `darwin` o `linux`. `arch`: `amd64` o `arm64`, según la compilación. `filename` debe ser un nombre simple terminado en `.zip` o `.tar.gz`. La URL debe ser HTTPS y no puede incluir credenciales. No se admiten mecanismos de autenticación propios ni scraping de páginas HTML.

Los identificadores internos se calculan a partir del repositorio y la URL; el manifiesto no necesita proporcionarlos. `localPath` del manifiesto remoto se ignora. Solo aparecen los paquetes compatibles con el sistema y arquitectura actuales. En un manifiesto propio el filtro de versión de Adoptium no aplica: el catálogo muestra todos sus paquetes compatibles.

El servidor debe responder un JSON de hasta 8 MiB. Se valida el esquema, los campos de todos los paquetes, los checksums y el acceso a un enlace compatible de muestra. La comprobación de muestra lee una respuesta acotada; cada descarga completa vuelve a verificar su integridad.

Los repositorios desactivados no permiten iniciar instalaciones. Las instalaciones ya en curso continúan aunque se desactive o quite su origen. El inventario conserva el identificador del repositorio original.

## Integridad y extracción

La firma de una actualización de tools-switcher y el SHA-256 de una JDK son mecanismos diferentes. Los checksums de JDKs se obtienen del proveedor o del manifiesto, que debe estar servido por una fuente de confianza.

Los paquetes se extraen a un directorio nuevo. Se rechazan rutas absolutas, escapes `..`, nombres con separadores Windows y tipos especiales no admitidos. Los enlaces TAR solo se aceptan si sus destinos resuelven dentro de la extracción. Los enlaces simbólicos ZIP se rechazan.

Una instalación requiere `release`, `JAVA_VERSION`, `bin/java` y `bin/javac` (con `.exe` en Windows). Las carpetas locales se importan mediante referencia y no se ejecutan para inspeccionarlas.
