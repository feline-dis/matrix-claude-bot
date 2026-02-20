{
  description = "Matrix bot powered by Claude";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.buildGoModule {
            pname = "matrix-claude-bot";
            version = "0.1.0";
            src = ./.;
            subPackages = [ "cmd/claude-bot" ];
            tags = [ "goolm" ];
            ldflags = [ "-s" "-w" ];

            vendorHash = "sha256-EE//SpdmJcFN2G4CIZ8GAX9eptVWzBW0OUaOkoCbR8o=";
          };
        });
    };
}
