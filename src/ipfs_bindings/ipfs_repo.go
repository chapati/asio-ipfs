package main

import (
    "log"
    "fmt"
    "os"
    "path/filepath"
    "github.com/ipfs/go-ipfs-config"
    "github.com/ipfs/interface-go-ipfs-core/options"
    "github.com/ipfs/go-ipfs/repo/fsrepo"
    "github.com/ipfs/go-ipfs/repo"
)

func initRepo(repoRoot string, asiocfg *AsioConfig) error {
    log.Print("Initializing IPFS node at %s\n", repoRoot)

    identity, err := config.CreateIdentity(os.Stdout, []options.KeyGenerateOption{
        options.Key.Type(options.Ed25519Key),
    })

    if err != nil {
        return err
    }

    conf, err := config.InitWithIdentity(identity)
    if err != nil {
        return err
    }

    if err := checkWritable(repoRoot); err != nil {
    	return err
    }

    if fsrepo.IsInitialized(repoRoot) {
        return fmt.Errorf("Repo already exists at %s", repoRoot)
    }

    //
    // Apply default profile to the config
    //
    if asiocfg.DefaultProfile != "" {
        log.Printf("Applying %s IPFS profile", asiocfg.DefaultProfile)
        transformer, ok := config.Profiles[asiocfg.DefaultProfile]

        if !ok {
            return fmt.Errorf("Unable to find default server profile.")
        }

        if err := transformer.Transform(conf); err != nil {
            return err
        }
    }

    //
    // And write our config values
    //
    if err = updateConfig(conf, asiocfg); err != nil {
        return err
    }

    if err = fsrepo.Init(repoRoot, conf); err != nil {
        return err
    }

    //
    // Write swarm.key to new repository
    //
    if err = updateRepo(repoRoot, asiocfg); err != nil {
        return err
    }

    // TODO:IPNS
    // return initializeIpnsKeyspace(repoRoot)
    return nil
}

func openRepo(repoRoot string, asiocfg *AsioConfig) (repo.Repo, error) {
    repo, err := fsrepo.Open(repoRoot)
    if err != nil {
        // TODO:NEXTVERIPFS Check for migration error and migrate
        return nil, err
    }

    //
    // Ensure that config passed from outside
    // is written into the IPFS repository
    //
    conf, err := repo.Config()
    if err != nil {
        return nil, err
    }

    conf, err = conf.Clone()
    if err != nil {
        return nil, err
    }

    if !configUnlocked(repoRoot) {
        log.Printf("WARNING: IPFS config is locked via %s. Config would be read from repository.\n", configLock)
        return repo, nil
    }

    if err = updateConfig(conf, asiocfg); err != nil {
        return nil, err
    }

    if err = repo.SetConfig(conf); err != nil {
        return nil, err
    }

    if err = updateRepo(repoRoot, asiocfg); err != nil {
        return nil, err
    }

    return repo, nil
}

func checkWritable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		testfile := filepath.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s, incorrect permissions", err)
	}

	return err
}