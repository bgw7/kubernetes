/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package create

import (
	"fmt"

	"github.com/spf13/cobra"

	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	batchv1beta1client "k8s.io/client-go/kubernetes/typed/batch/v1beta1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	cronjobLong = templates.LongDesc(`
		Create a cronjob with the specified name.`)

	cronjobExample = templates.Examples(`
		# Create a cronjob
		kubectl create cronjob my-job --image=busybox 

		# Create a cronjob with command
		kubectl create cronjob my-job --image=busybox -- date 

		# Create a cronjob with schedule
		kubectl create cronjob test-job --image=busybox --schedule="*/1 * * * *"`)
)

type CreateCronJobOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	PrintObj func(obj runtime.Object) error

	Name     string
	Image    string
	Schedule string
	Command  []string
	Restart  string

	Namespace string
	Client    batchv1beta1client.BatchV1beta1Interface
	DryRun    bool
	Builder   *resource.Builder
	Cmd       *cobra.Command

	genericclioptions.IOStreams
}

func NewCreateCronJobOptions(ioStreams genericclioptions.IOStreams) *CreateCronJobOptions {
	return &CreateCronJobOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(scheme.Scheme),
		IOStreams:  ioStreams,
	}
}

// NewCmdCreateCronJob is a command to create CronJobs.
func NewCmdCreateCronJob(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateCronJobOptions(ioStreams)
	cmd := &cobra.Command{
		Use:     "cronjob NAME --image=image --schedule='0/5 * * * ?' -- [COMMAND] [args...]",
		Aliases: []string{"cj"},
		Short:   cronjobLong,
		Long:    cronjobLong,
		Example: cronjobExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.PrintFlags.AddFlags(cmd)

	cmdutil.AddApplyAnnotationFlags(cmd)
	cmdutil.AddValidateFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
	cmd.Flags().StringVar(&o.Image, "image", o.Image, "Image name to run.")
	cmd.Flags().StringVar(&o.Schedule, "schedule", o.Schedule, "A schedule in the Cron format the job should be run with.")
	cmd.Flags().StringVar(&o.Restart, "restart", o.Restart, "job's restart policy. supported values: OnFailure, Never")

	return cmd
}

func (o *CreateCronJobOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	name, err := NameFromCommandArgs(cmd, args)
	if err != nil {
		return err
	}
	o.Name = name
	if len(args) > 1 {
		o.Command = args[1:]
	}
	if len(o.Restart) == 0 {
		o.Restart = "OnFailure"
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = batchv1beta1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Builder = f.NewBuilder()
	o.Cmd = cmd

	o.DryRun = cmdutil.GetDryRunFlag(cmd)
	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	o.PrintObj = func(obj runtime.Object) error {
		return printer.PrintObj(obj, o.Out)
	}

	return nil
}

func (o *CreateCronJobOptions) Validate() error {
	if len(o.Image) == 0 {
		return fmt.Errorf("--image must be specified")
	}
	if len(o.Schedule) == 0 {
		return fmt.Errorf("--schedule must be specified")
	}
	return nil
}

func (o *CreateCronJobOptions) Run() error {
	var cronjob *batchv1beta1.CronJob
	cronjob = o.createCronJob()

	if !o.DryRun {
		var err error
		cronjob, err = o.Client.CronJobs(o.Namespace).Create(cronjob)
		if err != nil {
			return fmt.Errorf("failed to create cronjob: %v", err)
		}
	}

	return o.PrintObj(cronjob)
}

func (o *CreateCronJobOptions) createCronJob() *batchv1beta1.CronJob {
	return &batchv1beta1.CronJob{
		TypeMeta: metav1.TypeMeta{APIVersion: batchv1beta1.SchemeGroupVersion.String(), Kind: "CronJob"},
		ObjectMeta: metav1.ObjectMeta{
			Name: o.Name,
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule: o.Schedule,
			JobTemplate: batchv1beta1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: o.Name,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    o.Name,
									Image:   o.Image,
									Command: o.Command,
								},
							},
							RestartPolicy: corev1.RestartPolicy(o.Restart),
						},
					},
				},
			},
		},
	}
}
