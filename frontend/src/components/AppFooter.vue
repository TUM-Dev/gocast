<template>
  <!--
    Ported from partial/footer.gohtml. Targets are still server-rendered pages, so
    these stay plain anchors.

    Two items from the template are missing: the version string and the optional wiki
    link. Both come from IndexData, which is interpolated into the template and has no
    API equivalent — a GetFrontendConfig RPC covering branding, version and wiki URL is
    the piece that brings them back.
  -->
  <footer id="desktop-footer" class="tum-live-footer hidden items-center justify-between md:flex">
    <div class="flex space-x-3">
      <a href="/about">About</a>
      <a href="/privacy">Data Privacy</a>
      <a href="/imprint">Imprint</a>
    </div>
    <a href="https://github.com/TUM-Dev/gocast" target="_blank" rel="noopener">
      gocast <i class="fa-brands fa-github"></i>
    </a>
  </footer>

  <footer class="tum-live-footer w-full md:hidden">
    <div class="grid divide-y dark:divide-gray-800">
      <a href="/about">About</a>
      <a href="/privacy">Data Privacy</a>
      <a href="/imprint">Imprint</a>
    </div>
    <a
      href="https://github.com/TUM-Dev/gocast"
      target="_blank"
      rel="noopener"
      class="mt-1 block text-center"
    >
      gocast <i class="fa-brands fa-github"></i>
    </a>
  </footer>
</template>
