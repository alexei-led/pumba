{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/24.05";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    flake-utils.url = "github:numtide/flake-utils";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  outputs =
    { self
    , nixpkgs
    , gomod2nix
    , flake-utils
    , treefmt-nix
    , ...
    }:
    flake-utils.lib.eachDefaultSystem (system:
    let
      pumba = with pkgs; gomod2nix.legacyPackages.${system}.buildGoApplication {
        name = "pumba";
        src = ./.;
        pwd = ./.; # Must be added due to bug https://github.com/nix-community/gomod2nix/issues/120

        # Rename the binary as it's called 'cmd' presently.
        fixupPhase = ''
          mv $out/bin/cmd $out/bin/pumba
        '';
      };

      goEnv = gomod2nix.legacyPackages.${system}.mkGoEnv { pwd = ./.; };

      pkgs = import nixpkgs { inherit system; };
    in
    {
      # Development shells for hacking
      devShells.default = pkgs.mkShell {
        packages = with pkgs; [
          goEnv
          gomod2nix.packages.${system}.default # gomod2nix CLI
        ];
      };

      packages = {
        default = pumba;
      };

      formatter =
        let
          fmt = treefmt-nix.lib.evalModule pkgs (_: {
            projectRootFile = "flake.nix";
            programs.nixpkgs-fmt.enable = true;
          });
        in
        fmt.config.build.wrapper;
    });
}
