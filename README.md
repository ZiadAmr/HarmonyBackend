# HarmonyBackend

## Docker commands

To build an image *harmony* and container *harmony0*:

    docker build -t harmony .
    docker run -d -p 8080:8080 --name harmony0 harmony