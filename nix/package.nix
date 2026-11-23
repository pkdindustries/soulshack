{
  lib,
  buildGoModule,
  bash,
}:

buildGoModule {
  pname = "soulshack";
  # A constant, not the commit: the version is part of this derivation's
  # name and of its dependency fetch's, so a commit hash here would download
  # every dependency again and rebuild on every commit, source changed or not.
  version = "0";

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../cmd
      ../internal
      ../examples/tools
    ];
  };

  vendorHash = "sha256-Dm14V8CtI0c2dJFj1FjuUVtGAyrjcsOLX7s0zuY6gI8=";

  subPackages = [ "cmd/soulshack" ];

  env.CGO_ENABLED = 0;
  ldflags = [
    "-s"
    "-w"
  ];

  # The example tools a config can name by path. The shell ones get their
  # #!/bin/bash rewritten to this bash (a NixOS host has no /bin/bash); what
  # they call (jq, curl), and python for the others, has to be on the
  # service's PATH.
  buildInputs = [ bash ];
  postInstall = ''
    install -Dm755 -t $out/share/soulshack/tools examples/tools/*
  '';

  meta.mainProgram = "soulshack";
}
