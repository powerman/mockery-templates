package faketest_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/powerman/check"

	"github.com/powerman/mockery-templates/internal/fixture"
	"github.com/powerman/mockery-templates/internal/fixture/a"
	"github.com/powerman/mockery-templates/internal/fixture/b"
	"github.com/powerman/mockery-templates/internal/fixture/faketest"
)

var (
	_ fixture.Repo            = (*faketest.FakeRepo)(nil)
	_ fixture.Variadic        = (*faketest.FakeVariadic)(nil)
	_ fixture.NoResults       = (*faketest.FakeNoResults)(nil)
	_ fixture.Collide         = (*faketest.FakeCollide)(nil)
	_ fixture.Swapper         = (*faketest.FakeSwapper)(nil)
	_ fixture.ReadCloser      = (*faketest.FakeReadCloser)(nil)
	_ fixture.I1[int]         = (*faketest.FakeI1[int])(nil)
	_ fixture.I2[int, string] = (*faketest.FakeI2[int, string])(nil)
	_ fixture.I3[int8]        = (*faketest.FakeI3[int8])(nil)
	_ fixture.I4[string]      = (*faketest.FakeI4[string])(nil)
	_ fixture.Cmp[string]     = (*faketest.FakeCmp[string])(nil)
	_ fixture.Empty           = (*faketest.FakeEmpty)(nil)
	_ fixture.AliasedRepo     = (*faketest.FakeRepo)(nil)
)

var errTest = errors.New("test")

type ctxKey struct{}

// repo is a hand-written fixture.Repo used to check delegation.
type repo struct{ calls []string }

func (r *repo) Get(_ fixture.Ctx, id int) (string, bool, error) {
	r.calls = append(r.calls, "Get")
	return "repo", id > 0, errTest
}

func (r *repo) Put(fixture.Ctx, string) error {
	r.calls = append(r.calls, "Put")
	return errTest
}

func (r *repo) Kinds(*int, []string, map[string]int, [4]byte, <-chan int, func(int) error) {
	r.calls = append(r.calls, "Kinds")
}

func (r *repo) Inline(s struct{ X int }, _ interface{ Y() }) struct{ X int } {
	r.calls = append(r.calls, "Inline")
	s.X++
	return s
}

type repoResults struct {
	get1   string
	get2   bool
	getErr error
	putErr error
	inline struct{ X int }
}

// callAllRepo invokes every method of fixture.Repo with fixed arguments.
func callAllRepo(ctx context.Context, r fixture.Repo) (res repoResults) {
	ctx = context.WithValue(ctx, ctxKey{}, "ctx")
	res.get1, res.get2, res.getErr = r.Get(ctx, 42)
	res.putErr = r.Put(ctx, "put")
	r.Kinds(nil, nil, nil, [4]byte{}, nil, nil)
	res.inline = r.Inline(struct{ X int }{X: 1}, nil)
	return res
}

// newRepoZeroResults returns a fake with every method configured to return zero values.
func newRepoZeroResults() *faketest.FakeRepo {
	fake := faketest.NewFakeRepo()
	fake.GetFunc.SetDefaultReturn("", false, nil)
	fake.PutFunc.SetDefaultReturn(nil)
	fake.KindsFunc.SetDefaultReturn()
	fake.InlineFunc.SetDefaultReturn(struct{ X int }{})
	return fake
}

func TestNewFakeRepo(tt *testing.T) {
	tt.Parallel()

	tests := map[string]*faketest.FakeRepo{
		"constructor": faketest.NewFakeRepo(),
		"zero value":  {},
	}
	for name, fake := range tests {
		tt.Run(name, func(tt *testing.T) {
			tt.Parallel()
			t := check.T(tt)

			t.PanicMatch(func() { _, _, _ = fake.Get(t.Context(), 0) },
				`^unexpected invocation of FakeRepo.Get$`)
			t.PanicMatch(func() { _ = fake.Put(t.Context(), "") },
				`^unexpected invocation of FakeRepo.Put$`)
			t.PanicMatch(func() { fake.Kinds(nil, nil, nil, [4]byte{}, nil, nil) },
				`^unexpected invocation of FakeRepo.Kinds$`)
			t.PanicMatch(func() { _ = fake.Inline(struct{ X int }{}, nil) },
				`^unexpected invocation of FakeRepo.Inline$`)

			fake.PutFunc.SetDefaultReturn(errTest)
			t.Err(fake.Put(t.Context(), ""), errTest)
			t.Len(fake.GetFunc.History(), 0)
			t.Len(fake.PutFunc.History(), 1)
		})
	}
}

func TestNewFakeRepo_configured(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	fake := newRepoZeroResults()
	t.Zero(callAllRepo(t.Context(), fake))

	getCalls := fake.GetFunc.History()
	t.Len(getCalls, 1)
	t.Equal(getCalls[0].Ctx.Value(ctxKey{}), "ctx")
	t.Equal(getCalls[0].ID, 42)
	t.Zero(getCalls[0].S)
	t.False(getCalls[0].B)
	t.Nil(getCalls[0].Err)
	t.Len(getCalls[0].Args(), 2)
	t.DeepEqual(getCalls[0].Results(), []any{"", false, nil})

	putCalls := fake.PutFunc.History()
	t.Len(putCalls, 1)
	t.Equal(putCalls[0].S, "put")
	t.DeepEqual(putCalls[0].Results(), []any{error(nil)})

	kindsCalls := fake.KindsFunc.History()
	t.Len(kindsCalls, 1)
	t.Len(kindsCalls[0].Args(), 6)
	t.DeepEqual(kindsCalls[0].Results(), []any{})

	inlineCalls := fake.InlineFunc.History()
	t.Len(inlineCalls, 1)
	t.Equal(inlineCalls[0].S.X, 1)
	t.Zero(inlineCalls[0].Val)
}

func TestNewFakeRepo_drainedQueue(tt *testing.T) {
	tt.Parallel()

	tests := map[string]*faketest.FakeRepo{
		"constructor": faketest.NewFakeRepo(),
		"zero value":  {},
	}
	for name, fake := range tests {
		tt.Run(name, func(tt *testing.T) {
			tt.Parallel()
			t := check.T(tt).MustAll()

			fake.PutFunc.PushReturn(errTest)
			fake.PutFunc.PushHook(func(context.Context, string) error { return nil })
			t.Err(fake.Put(t.Context(), ""), errTest)
			t.Nil(fake.Put(t.Context(), ""))
			t.PanicMatch(func() { _ = fake.Put(t.Context(), "") },
				`^unexpected invocation of FakeRepo.Put$`)
			t.Len(fake.PutFunc.History(), 2)

			// The panic must not leave the mutex locked.
			fake.PutFunc.SetDefaultHook(func(context.Context, string) error { return errTest })
			t.Err(fake.Put(t.Context(), ""), errTest)
			t.Len(fake.PutFunc.History(), 3)
		})
	}
}

func TestNewFakeRepoFrom(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	impl := &repo{}
	fake := faketest.NewFakeRepoFrom(impl)
	res := callAllRepo(t.Context(), fake)
	t.Equal(res.get1, "repo")
	t.True(res.get2)
	t.Err(res.getErr, errTest)
	t.Err(res.putErr, errTest)
	t.Equal(res.inline.X, 2)
	t.DeepEqual(impl.calls, []string{"Get", "Put", "Kinds", "Inline"})
	t.Err(fake.GetFunc.History()[0].Err, errTest)

	fake.GetFunc.PushReturn("pushed", false, nil)
	get1, _, getErr := fake.Get(t.Context(), 1)
	t.Equal(get1, "pushed")
	t.Nil(getErr)
	get1, _, _ = fake.Get(t.Context(), 1)
	t.Equal(get1, "repo")
}

func TestFakeRepoGetFunc_PushHook(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	fake := faketest.NewFakeRepo()
	fake.GetFunc.SetDefaultReturn("", false, nil)
	for _, s := range []string{"one", "two"} {
		fake.GetFunc.PushHook(func(_ context.Context, id int) (string, bool, error) {
			return s, id > 0, nil
		})
	}
	fake.GetFunc.PushReturn("three", false, errTest)

	get := func() (string, bool, error) { return fake.Get(t.Context(), 1) }
	assertGet(t, get, "one", true, nil)
	assertGet(t, get, "two", true, nil)
	assertGet(t, get, "three", false, errTest)
	assertGet(t, get, "", false, nil)
	assertGet(t, get, "", false, nil)

	fake.GetFunc.SetDefaultReturn("default", true, nil)
	fake.GetFunc.PushReturn("pushed", false, nil)
	assertGet(t, get, "pushed", false, nil)
	assertGet(t, get, "default", true, nil)
	assertGet(t, get, "default", true, nil)

	fake.GetFunc.SetDefaultHook(func(_ context.Context, id int) (string, bool, error) {
		return "hook", id < 0, errTest
	})
	assertGet(t, get, "hook", false, errTest)

	t.Len(fake.GetFunc.History(), 9)
	t.Equal(fake.GetFunc.History()[2].S, "three")
}

func assertGet(t *check.C, get func() (string, bool, error), want1 string, want2 bool, wantErr error) {
	t.Helper()
	got1, got2, err := get()
	t.Equal(got1, want1)
	t.Equal(got2, want2)
	t.Err(err, wantErr)
}

func TestFakeRepoGetFunc_History(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	fake := faketest.NewFakeRepo()
	fake.GetFunc.SetDefaultReturn("", false, nil)
	_, _, _ = fake.Get(t.Context(), 1)
	_, _, _ = fake.Get(t.Context(), 2)

	history := fake.GetFunc.History()
	t.Len(history, 2)
	t.Equal(history[0].ID, 1)
	t.Equal(history[1].ID, 2)
	history[0].ID = 10
	history[1].ID = 20
	t.Equal(fake.GetFunc.History()[0].ID, 1)
	t.Equal(fake.GetFunc.History()[1].ID, 2)
}

func TestVariadic(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	fake := faketest.NewFakeVariadic()
	fake.DoFunc.SetDefaultHook(func(command string, args ...any) (any, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%w: %s", errTest, command)
		}
		return len(args), nil
	})
	res, err := fake.Do("cmd", 1, "two", nil)
	t.Equal(res, 3)
	t.Nil(err)
	_, err = fake.Do("none")
	t.Match(err, "none")
	fake.OnlyFunc.SetDefaultReturn(nil)
	t.Nil(fake.Only())
	opt1 := fixture.Option(func(*int) {})
	opt2 := fixture.Option(func(*int) {})
	fake.OnlyFunc.PushReturn(errTest)
	t.Err(fake.Only(opt1, opt2), errTest)

	doCalls := fake.DoFunc.History()
	t.Len(doCalls, 2)
	t.Equal(doCalls[0].Command, "cmd")
	t.DeepEqual(doCalls[0].ArgArgs, []any{1, "two", nil})
	t.DeepEqual(doCalls[0].Args(), []any{"cmd", 1, "two", nil})
	t.Equal(doCalls[0].Results()[0], 3)
	t.Len(doCalls[0].Results(), 2)
	t.Len(doCalls[1].ArgArgs, 0)
	t.DeepEqual(doCalls[1].Args(), []any{"none"})

	onlyCalls := fake.OnlyFunc.History()
	t.Len(onlyCalls, 2)
	t.Len(onlyCalls[0].Opts, 0)
	t.DeepEqual(onlyCalls[0].Args(), []any{})
	t.Len(onlyCalls[1].Opts, 2)
	t.Len(onlyCalls[1].Args(), 2)
	t.DeepEqual(onlyCalls[1].Results(), []any{errTest})
}

func TestNoResults(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	fake := faketest.NewFakeNoResults()
	t.PanicMatch(func() { fake.Fire(1) }, `^unexpected invocation of FakeNoResults.Fire$`)
	t.PanicMatch(fake.Nothing, `^unexpected invocation of FakeNoResults.Nothing$`)

	var fired []int
	fake.FireFunc.SetDefaultHook(func(v int) { fired = append(fired, v) })
	fake.FireFunc.PushReturn()
	fake.Fire(1)
	fake.Fire(2)
	fake.NothingFunc.SetDefaultReturn()
	fake.Nothing()
	t.DeepEqual(fired, []int{2})

	fireCalls := fake.FireFunc.History()
	t.Len(fireCalls, 2)
	t.DeepEqual(fireCalls[0].Args(), []any{1})
	t.DeepEqual(fireCalls[1].Results(), []any{})
	nothingCalls := fake.NothingFunc.History()
	t.Len(nothingCalls, 1)
	t.DeepEqual(nothingCalls[0].Args(), []any{})
	t.DeepEqual(nothingCalls[0].Results(), []any{})
}

func TestCollide(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	fake := faketest.NewFakeCollide()
	fake.NamesFunc.PushReturn(7, errTest)
	var mutex sync.Mutex
	r1, err := fake.Names(1, 2, 3, errTest, "v0", &mutex, []int{4}, true, "i")
	t.Equal(r1, 7)
	t.Err(err, errTest)
	t.DeepEqual(fake.NamesFunc.History()[0].Args(),
		[]any{1, 2, 3, errTest, "v0", &mutex, []int{4}, true, "i"})
}

func TestSwapper(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	fake := faketest.NewFakeSwapper()
	fake.SwapFunc.SetDefaultReturn(b.Thing{})
	t.Equal(fake.Swap(a.Thing{}), b.Thing{})
	t.Equal(fake.SwapFunc.History()[0].T, a.Thing{})
}

func TestReadCloser(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	fake := faketest.NewFakeReadCloser()
	fake.ReadFunc.PushReturn(3, nil)
	fake.CloseFunc.SetDefaultReturn(errTest)
	n, err := fake.Read(make([]byte, 4))
	t.Equal(n, 3)
	t.Nil(err)
	t.Err(fake.Close(), errTest)
}

func TestGenerics(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	i2 := faketest.NewFakeI2[int, string]()
	i2.M1Func.SetDefaultReturn()
	i2.M2Func.SetDefaultReturn("")
	i2.M2Func.PushReturn("two")
	i2.M1(1)
	t.Equal(i2.M2(2), "two")
	t.Equal(i2.M2(3), "")
	t.Equal(i2.M1Func.History()[0].V, 1)
	t.Equal(i2.M2Func.History()[1].V, 3)

	i2From := faketest.NewFakeI2From[int, string](i2)
	t.Equal(i2From.M2(4), "")
	t.Len(i2.M2Func.History(), 3)

	cmp := faketest.NewFakeCmp[string]()
	cmp.EqFunc.SetDefaultHook(func(x, y string) bool { return x == y })
	t.True(cmp.Eq("a", "a"))
	t.False(cmp.Eq("a", "b"))
	t.DeepEqual(cmp.EqFunc.History()[1].Args(), []any{"a", "b"})

	i3 := faketest.NewFakeI3[int]()
	t.PanicMatch(func() { _ = i3.M3(nil) }, `^unexpected invocation of FakeI3.M3$`)
	i3.M3Func.SetDefaultReturn(3)
	t.Equal(i3.M3(faketest.NewFakeI1[int]()), 3)

	var i4 faketest.FakeI4[string]
	t.PanicMatch(func() { _ = i4.M4(nil) }, `^unexpected invocation of FakeI4.M4$`)
	i4.M4Func.PushReturn(4)
	t.Equal(i4.M4(nil), 4)
}

func TestEmpty(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt)

	t.NotNil(faketest.NewFakeEmpty())
	t.NotNil(faketest.NewFakeEmptyFrom(nil))
}

func TestCallFields(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	fake := faketest.NewFakeCallFields()

	fake.NamedFunc.PushReturn(3, errTest)
	_, _ = fake.Named("name", 1)
	named := fake.NamedFunc.History()[0]
	t.DeepEqual(named, faketest.FakeCallFieldsNamedFuncCall{Name: "name", Perm: 1, Size: 3, Err: errTest})
	t.DeepEqual(named.Args(), []any{"name", 1})
	t.DeepEqual(named.Results(), []any{3, errTest})

	thing := a.Thing{}
	fake.UnnamedFunc.PushReturn("r", 2, errTest)
	_, _, _ = fake.Unnamed(1, "s", "any", thing, true, false)
	unnamed := fake.UnnamedFunc.History()[0]
	t.DeepEqual(unnamed, faketest.FakeCallFieldsUnnamedFuncCall{
		N: 1, S: "s", Any: "any", Thing: thing, Bools: []bool{true, false},
		S1: "r", Any1: 2, Err: errTest,
	})
	t.DeepEqual(unnamed.Args(), []any{1, "s", "any", thing, true, false})
	t.DeepEqual(unnamed.Results(), []any{"r", 2, errTest})

	fake.BlankFunc.PushReturn(true, errTest)
	_, _ = fake.Blank(1, "s")
	t.DeepEqual(fake.BlankFunc.History()[0], faketest.FakeCallFieldsBlankFuncCall{
		N: 1, S: "s", B: true, Err: errTest,
	})

	fake.ShortFunc.PushReturn(3, 4)
	_, _ = fake.Short("s", 1, nil, struct{}{})
	t.DeepEqual(fake.ShortFunc.History()[0], faketest.FakeCallFieldsShortFuncCall{
		S: "s", N: 1, Fn: nil, Val: struct{}{}, X: 3, Y: 4,
	})

	fake.MethodsFunc.PushReturn()
	fake.Methods(1, 2)
	methods := fake.MethodsFunc.History()[0]
	t.DeepEqual(methods, faketest.FakeCallFieldsMethodsFuncCall{ArgArgs: 1, ArgResults: 2})
	t.DeepEqual(methods.Args(), []any{1, 2})
	t.DeepEqual(methods.Results(), []any{})

	fake.ResultMethodsFunc.PushReturn(1, 2)
	_, _ = fake.ResultMethods()
	resultMethods := fake.ResultMethodsFunc.History()[0]
	t.DeepEqual(resultMethods,
		faketest.FakeCallFieldsResultMethodsFuncCall{ResultArgs: 1, ResultResults: 2})
	t.DeepEqual(resultMethods.Results(), []any{1, 2})

	fake.PrefixedFunc.PushReturn(4)
	_ = fake.Prefixed(1, 2, 3)
	t.DeepEqual(fake.PrefixedFunc.History()[0],
		faketest.FakeCallFieldsPrefixedFuncCall{X: 1, Arg0: 2, Result0: 3, Arg1: 4})

	fake.SameCaseFunc.PushReturn()
	fake.SameCase(1, 2, 3)
	t.DeepEqual(fake.SameCaseFunc.History()[0],
		faketest.FakeCallFieldsSameCaseFuncCall{Foo: 1, ArgFoo: 2, ArgArgFoo: 3})

	fake.ArgResultFunc.PushReturn(errTest)
	_ = fake.ArgResult(1)
	t.DeepEqual(fake.ArgResultFunc.History()[0],
		faketest.FakeCallFieldsArgResultFuncCall{Size: 1, ResultSize: errTest})

	fake.ErrCaseFunc.PushReturn(errTest)
	_ = fake.ErrCase(1)
	t.DeepEqual(fake.ErrCaseFunc.History()[0],
		faketest.FakeCallFieldsErrCaseFuncCall{Err: 1, ResultErr: errTest})

	var mutex sync.Mutex
	now := time.Now()
	fake.ImportedFunc.PushReturn(now)
	_ = fake.Imported(t.Context(), &mutex)
	imported := fake.ImportedFunc.History()[0]
	t.Equal(imported.Context, t.Context())
	t.Equal(imported.Sync, &mutex)
	t.Equal(imported.Time, now)

	fake.NumberedFunc.PushReturn()
	fake.Numbered(1, 2, 3)
	t.DeepEqual(fake.NumberedFunc.History()[0],
		faketest.FakeCallFieldsNumberedFuncCall{X1: 1, X: 2, V2: 3})

	fake.NotExportedFunc.PushReturn()
	fake.NotExported(1, 2, 3)
	t.DeepEqual(fake.NotExportedFunc.History()[0],
		faketest.FakeCallFieldsNotExportedFuncCall{Arg_x: 1, Arg名: 2, Ключ: 3})
}

func TestConcurrent(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	const workers = 8
	const calls = 100
	fake := faketest.NewFakeRepo()
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range calls {
				fake.PutFunc.PushReturn(errTest)
			}
		})
	}
	wg.Wait()
	for range workers {
		wg.Go(func() {
			for range calls {
				_ = fake.Put(t.Context(), "")
				_ = fake.PutFunc.History()
			}
		})
	}
	wg.Wait()

	history := fake.PutFunc.History()
	t.Len(history, workers*calls)
	for _, call := range history {
		t.Err(call.Err, errTest)
	}
	t.PanicMatch(func() { _ = fake.Put(t.Context(), "") },
		`^unexpected invocation of FakeRepo.Put$`)
}
