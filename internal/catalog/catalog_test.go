package catalog

import (
	"strings"
	"testing"
)

func TestResolveExpandsMetricSetBindings(t *testing.T) {
	input := `{
	  "version": 1,
	  "resources": [
	    {
	      "key": "primary-ec2",
	      "serviceName": "ec2",
	      "resourceIdEnv": "TARGET_INSTANCE_ID",
	      "regionEnv": "AWS_REGION"
	    }
	  ],
	  "metricSets": [
	    {
	      "name": "ec2-default",
	      "metrics": [
	        {
	          "namespace": "AWS/EC2",
	          "metricName": "CPUUtilization",
	          "statistic": "Average",
	          "periodSeconds": 300,
	          "unit": "Percent"
	        },
	        {
	          "serviceName": "cwagent",
	          "namespace": "CWAgent",
	          "metricName": "mem_used_percent",
	          "statistic": "Average",
	          "periodSeconds": 300,
	          "unit": "Percent"
	        }
	      ]
	    }
	  ],
	  "bindings": [
	    {
	      "resourceKey": "primary-ec2",
	      "metricSet": "ec2-default",
	      "enabled": true
	    }
	  ]
	}`

	c, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	defs, err := c.Resolve(func(key string) string {
		values := map[string]string{
			"TARGET_INSTANCE_ID": "i-REPLACE_WITH_INSTANCE_ID",
			"AWS_REGION":         "us-east-1",
		}
		return values[key]
	})
	if err != nil {
		t.Fatalf("resolve catalog: %v", err)
	}

	if got, want := len(defs), 2; got != want {
		t.Fatalf("definition count = %d, want %d", got, want)
	}

	if defs[0].ResourceID != "i-REPLACE_WITH_INSTANCE_ID" {
		t.Fatalf("resource id = %q", defs[0].ResourceID)
	}

	var foundCWAgent bool
	for _, def := range defs {
		if def.Namespace == "CWAgent" {
			foundCWAgent = true
			if def.ServiceName != "cwagent" {
				t.Fatalf("CWAgent service name = %q, want cwagent", def.ServiceName)
			}
		}
	}
	if !foundCWAgent {
		t.Fatal("expected CWAgent definition")
	}
}

func TestResolveRejectsMissingEnvironmentValue(t *testing.T) {
	input := `{
	  "version": 1,
	  "resources": [
	    {
	      "key": "primary-ec2",
	      "serviceName": "ec2",
	      "resourceIdEnv": "TARGET_INSTANCE_ID",
	      "regionEnv": "AWS_REGION"
	    }
	  ],
	  "metricSets": [
	    {
	      "name": "ec2-default",
	      "metrics": [
	        {
	          "namespace": "AWS/EC2",
	          "metricName": "CPUUtilization",
	          "statistic": "Average",
	          "periodSeconds": 300
	        }
	      ]
	    }
	  ],
	  "bindings": [
	    {
	      "resourceKey": "primary-ec2",
	      "metricSet": "ec2-default"
	    }
	  ]
	}`

	c, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	if _, err := c.Resolve(func(string) string { return "" }); err == nil {
		t.Fatal("expected missing environment validation error")
	}
}

func TestValidateRejectsUnknownBindingReference(t *testing.T) {
	input := `{
	  "version": 1,
	  "resources": [
	    {
	      "key": "primary-ec2",
	      "serviceName": "ec2",
	      "resourceIdEnv": "TARGET_INSTANCE_ID",
	      "regionEnv": "AWS_REGION"
	    }
	  ],
	  "metricSets": [
	    {
	      "name": "ec2-default",
	      "metrics": [
	        {
	          "namespace": "AWS/EC2",
	          "metricName": "CPUUtilization",
	          "statistic": "Average",
	          "periodSeconds": 300
	        }
	      ]
	    }
	  ],
	  "bindings": [
	    {
	      "resourceKey": "missing",
	      "metricSet": "ec2-default"
	    }
	  ]
	}`

	if _, err := Load(strings.NewReader(input)); err == nil {
		t.Fatal("expected unknown binding resource error")
	}
}
