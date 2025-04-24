package main

import (
	"context"
	"fmt"

	"github.com/loog-project/loog/internal/service"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func Must2[T any](v T, err error) T {
	Must(err)
	return v
}

func main() {
	ctx := context.TODO()

	rps, err := bboltStore.New("test.bb", nil)
	if err != nil {
		panic(err)
	}
	ts := service.NewTrackerService(rps, 2)

	data := &unstructured.Unstructured{
		Object: make(map[string]interface{}),
	}
	data.SetNamespace("default")
	data.SetName("test")
	data.SetUID("ffffffff-ffff-ffff-ffff-fffffffffff2")

	uid := string(data.GetUID())

	rev1 := Must2(ts.Commit(ctx, uid, data))
	snap1 := Must2(ts.Restore(ctx, uid, rev1))
	fmt.Println("Revision 1:", rev1, "val:", snap1.Object)

	data.SetName("new-name")
	rev2 := Must2(ts.Commit(ctx, uid, data))
	snap2 := Must2(ts.Restore(ctx, uid, rev2))
	fmt.Println("Revision 2:", rev2, "val:", snap2.Object)

	data.SetNamespace("new-namespace")
	rev3 := Must2(ts.Commit(ctx, uid, data))
	snap3 := Must2(ts.Restore(ctx, uid, rev3))
	fmt.Println("Revision 3:", rev3, "val:", snap3.Object)

	data.SetAnnotations(map[string]string{
		"hello": "world",
	})
	data.SetGeneration(11)
	rev4 := Must2(ts.Commit(ctx, uid, data))
	snap4 := Must2(ts.Restore(ctx, uid, rev4))
	fmt.Println("Revision 4:", rev4, "val:", snap4.Object)

	fmt.Println("Saved everything!")
}
