# tools-switcher

Primera versión del gestor de herramientas de desarrollo, con soporte para JDKs:

- Catálogos Temurin, manifiestos HTTPS y carpetas offline.
- Descarga múltiple, verificación SHA-256, importación y gestión de JAVA_HOME/PATH.
- Temas claro/oscuro/sistema, vault configurable y bandeja.
- Actualizaciones verificadas mediante firma digital.

## Instalación

Windows: usa el instalador `-setup.exe`, que instala para el usuario actual y contempla WebView2.

macOS ARM64: abre el DMG y copia la aplicación a Aplicaciones. La aplicación tiene firma ad hoc y no está notarizada; puede requerir autorización manual en Privacidad y seguridad.

Linux x64: usa DEB/RPM para instalar con el gestor de paquetes o el archivo portable en un directorio de usuario. Necesita GTK4 y WebKitGTK 6.0. Los paquetes del sistema se actualizan mediante su gestor de paquetes.

Abre una terminal nueva después de cambiar variables de entorno.
