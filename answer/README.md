# Answers

Here is some documentation talking about some of the things that I learned were necessary to get this project off the ground. 

## Understanding the Project

First we have to know what we are dealing with.

`/cmds/my-app-controller/main.go` - this holds the start to the controller. Not too much to see in here, but it is the starting point to our controller.

`/pkg/controller/controller.go` - in here is where the controller code is defined. This has the all important `reconcile` function which is how our controller will act upon our custom resource.

`pkg/api/app.go` - in here we have the ability to register the Kind in our controller to have the ability to process it. Fairly straight forward. 


`/configs` we have two files in here
- `app.yaml` which is our basic definition of the custom resource which has been given to us
- `example.yaml` which is three different instantiations of the custom resource kind that is used for testing our controller.

## Makefile

Lets try and understand whats going on in this makefile to get a better sense on how this and other projects may build & test their controllers.

Kind is used as a way to test a local deployment of kubernetes. This will set up a local cluster which will spin up pods for you.

`make deploy-kind` will get you most of the way there. This will help you deploy the existing configs on top of set up the kind workspace to get started. 

This will also push the image to your local docker desktop which you should be able to interact with pretty seamlessly in the project.

## Getting around the cluster

For the most part everything will be set up in the default namespace so nothing weird there.

`k get pods` should do it once we learn how to deploy our controller...

## How to deploy the controller

We have a bunch of controller code but no way to deploy it. What do we need? Theres a few kubernetes yamls that are necessary at this step.

`deployment.yaml` - this has the regular kube deployment spec which will spin up the controller. 

`service_account.yaml` - this is necessary for your controller to assume the right permissions to act within the cluster.

`role.yaml` - this defines the permissions that are necessary for the service_account to assume

`role_binding.yaml` - this is necessary to bind the role permissions to the service account.

We need to make all four of this files and have them be part of our `make deploy-kind` in order to get off the ground. So I just shoved them all in configs to make things easier.

At this point your controller should be up and running. You can check the pod running the controller. Some simple logs can be set up by using the `log.SetLogger()`. Now we have a controller wired up to listen to `MyApp` kind resources.

## How to configure the deployment based on what is defined in the MyApp Kind

Now we are in deep in the controller code. The `MyApp{}` struct should contain all the details that are in the manifest files given in `example.yaml`. We were constructed to make a new deployment so we will have to create a `Deployment` struct within our Reconcile method. In the `Reconcile` method we can handle all of the native kubernetes build / clean up as that is where kubernetes will go to fix the state of the world.

Adding a PDB is very similar. Nothing too out of the blue once we are already up and running.

## How to verify the metrics agent

We learned how to expose the metrics agent which is default on port 8080. We can then run a port-forwarding command to verify for ourselves.

`k port-forward my-app-controller-c84cd7b5b-nstpk 8080:8080 `

We then can check `localhost:8080/metrics` and see all of our metrics live!
