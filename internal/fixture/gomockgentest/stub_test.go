package gomockgentest_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/mockery-templates/internal/fixture"
	"github.com/powerman/mockery-templates/internal/fixture/a"
	"github.com/powerman/mockery-templates/internal/fixture/b"
	"github.com/powerman/mockery-templates/internal/fixture/gomockgentest"
)

var (
	_ fixture.Repo            = gomockgentest.NewMockRepo()
	_ fixture.Variadic        = gomockgentest.NewMockVariadic()
	_ fixture.NoResults       = gomockgentest.NewMockNoResults()
	_ fixture.Collide         = gomockgentest.NewMockCollide()
	_ fixture.Swapper         = gomockgentest.NewMockSwapper()
	_ fixture.ReadCloser      = gomockgentest.NewMockReadCloser()
	_ fixture.I1[int]         = gomockgentest.NewMockI1[int]()
	_ fixture.I2[int, string] = gomockgentest.NewMockI2[int, string]()
	_ fixture.I3[int8]        = gomockgentest.NewMockI3[int8]()
	_ fixture.I4[string]      = gomockgentest.NewMockI4[string]()
	_ fixture.Cmp[string]     = gomockgentest.NewMockCmp[string]()
	_ fixture.Empty           = gomockgentest.NewMockEmpty()
	_ fixture.Repo            = gomockgentest.NewStrictMockRepo()
	_ fixture.I2[int, string] = gomockgentest.NewStrictMockI2[int, string]()
	_ fixture.Empty           = gomockgentest.NewMockEmptyFrom(nil)
	_ fixture.Empty           = gomockgentest.NewStrictMockEmpty()
	_ fixture.AliasedRepo     = gomockgentest.NewMockRepo()
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

func TestNewMockRepo(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	mock := gomockgentest.NewMockRepo()
	t.Zero(callAllRepo(t.Context(), mock))

	getCalls := mock.GetFunc.History()
	t.Len(getCalls, 1)
	t.Equal(getCalls[0].Arg0.Value(ctxKey{}), "ctx")
	t.Equal(getCalls[0].Arg1, 42)
	t.Zero(getCalls[0].Result0)
	t.False(getCalls[0].Result1)
	t.Nil(getCalls[0].Result2)
	t.Len(getCalls[0].Args(), 2)
	t.DeepEqual(getCalls[0].Results(), []any{"", false, nil})

	putCalls := mock.PutFunc.History()
	t.Len(putCalls, 1)
	t.Equal(putCalls[0].Arg1, "put")
	t.DeepEqual(putCalls[0].Results(), []any{error(nil)})

	kindsCalls := mock.KindsFunc.History()
	t.Len(kindsCalls, 1)
	t.Len(kindsCalls[0].Args(), 6)
	t.DeepEqual(kindsCalls[0].Results(), []any{})

	inlineCalls := mock.InlineFunc.History()
	t.Len(inlineCalls, 1)
	t.Equal(inlineCalls[0].Arg0.X, 1)
	t.Zero(inlineCalls[0].Result0)
}

func TestNewStrictMockRepo(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt)

	mock := gomockgentest.NewStrictMockRepo()
	t.PanicMatch(func() { _, _, _ = mock.Get(t.Context(), 0) },
		`^unexpected invocation of MockRepo.Get$`)
	t.PanicMatch(func() { _ = mock.Put(t.Context(), "") },
		`^unexpected invocation of MockRepo.Put$`)
	t.PanicMatch(func() { mock.Kinds(nil, nil, nil, [4]byte{}, nil, nil) },
		`^unexpected invocation of MockRepo.Kinds$`)
	t.PanicMatch(func() { _ = mock.Inline(struct{ X int }{}, nil) },
		`^unexpected invocation of MockRepo.Inline$`)

	mock.PutFunc.SetDefaultReturn(errTest)
	t.Err(mock.Put(t.Context(), ""), errTest)
	t.Len(mock.GetFunc.History(), 0)
	t.Len(mock.PutFunc.History(), 1)
}

func TestNewMockRepoFrom(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	impl := &repo{}
	mock := gomockgentest.NewMockRepoFrom(impl)
	res := callAllRepo(t.Context(), mock)
	t.Equal(res.get1, "repo")
	t.True(res.get2)
	t.Err(res.getErr, errTest)
	t.Err(res.putErr, errTest)
	t.Equal(res.inline.X, 2)
	t.DeepEqual(impl.calls, []string{"Get", "Put", "Kinds", "Inline"})
	t.Err(mock.GetFunc.History()[0].Result2, errTest)

	mock.GetFunc.PushReturn("pushed", false, nil)
	get1, _, getErr := mock.Get(t.Context(), 1)
	t.Equal(get1, "pushed")
	t.Nil(getErr)
	get1, _, _ = mock.Get(t.Context(), 1)
	t.Equal(get1, "repo")
}

func TestRepoGetFunc_PushHook(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	mock := gomockgentest.NewMockRepo()
	for _, s := range []string{"one", "two"} {
		mock.GetFunc.PushHook(func(_ context.Context, id int) (string, bool, error) {
			return s, id > 0, nil
		})
	}
	mock.GetFunc.PushReturn("three", false, errTest)

	get := func() (string, bool, error) { return mock.Get(t.Context(), 1) }
	assertGet(t, get, "one", true, nil)
	assertGet(t, get, "two", true, nil)
	assertGet(t, get, "three", false, errTest)
	assertGet(t, get, "", false, nil)
	assertGet(t, get, "", false, nil)

	mock.GetFunc.SetDefaultReturn("default", true, nil)
	mock.GetFunc.PushReturn("pushed", false, nil)
	assertGet(t, get, "pushed", false, nil)
	assertGet(t, get, "default", true, nil)
	assertGet(t, get, "default", true, nil)

	mock.GetFunc.SetDefaultHook(func(_ context.Context, id int) (string, bool, error) {
		return "hook", id < 0, errTest
	})
	assertGet(t, get, "hook", false, errTest)

	t.Len(mock.GetFunc.History(), 9)
	t.Equal(mock.GetFunc.History()[2].Result0, "three")
}

func assertGet(t *check.C, get func() (string, bool, error), want1 string, want2 bool, wantErr error) {
	t.Helper()
	got1, got2, err := get()
	t.Equal(got1, want1)
	t.Equal(got2, want2)
	t.Err(err, wantErr)
}

func TestRepoGetFunc_History(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	mock := gomockgentest.NewMockRepo()
	_, _, _ = mock.Get(t.Context(), 1)
	_, _, _ = mock.Get(t.Context(), 2)

	history := mock.GetFunc.History()
	t.Len(history, 2)
	t.Equal(history[0].Arg1, 1)
	t.Equal(history[1].Arg1, 2)
	history[0].Arg1 = 10
	history[1].Arg1 = 20
	t.Equal(mock.GetFunc.History()[0].Arg1, 1)
	t.Equal(mock.GetFunc.History()[1].Arg1, 2)
}

func TestVariadic(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	mock := gomockgentest.NewMockVariadic()
	mock.DoFunc.SetDefaultHook(func(command string, args ...any) (any, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%w: %s", errTest, command)
		}
		return len(args), nil
	})
	res, err := mock.Do("cmd", 1, "two", nil)
	t.Equal(res, 3)
	t.Nil(err)
	_, err = mock.Do("none")
	t.Match(err, "none")
	t.Nil(mock.Only())
	opt1 := fixture.Option(func(*int) {})
	opt2 := fixture.Option(func(*int) {})
	mock.OnlyFunc.PushReturn(errTest)
	t.Err(mock.Only(opt1, opt2), errTest)

	doCalls := mock.DoFunc.History()
	t.Len(doCalls, 2)
	t.Equal(doCalls[0].Arg0, "cmd")
	t.DeepEqual(doCalls[0].Arg1, []any{1, "two", nil})
	t.DeepEqual(doCalls[0].Args(), []any{"cmd", 1, "two", nil})
	t.Equal(doCalls[0].Results()[0], 3)
	t.Len(doCalls[0].Results(), 2)
	t.Len(doCalls[1].Arg1, 0)
	t.DeepEqual(doCalls[1].Args(), []any{"none"})

	onlyCalls := mock.OnlyFunc.History()
	t.Len(onlyCalls, 2)
	t.Len(onlyCalls[0].Arg0, 0)
	t.DeepEqual(onlyCalls[0].Args(), []any{})
	t.Len(onlyCalls[1].Arg0, 2)
	t.Len(onlyCalls[1].Args(), 2)
	t.DeepEqual(onlyCalls[1].Results(), []any{errTest})
}

func TestNoResults(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	mock := gomockgentest.NewStrictMockNoResults()
	t.PanicMatch(func() { mock.Fire(1) }, `^unexpected invocation of MockNoResults.Fire$`)
	t.PanicMatch(mock.Nothing, `^unexpected invocation of MockNoResults.Nothing$`)

	var fired []int
	mock.FireFunc.SetDefaultHook(func(v int) { fired = append(fired, v) })
	mock.FireFunc.PushReturn()
	mock.Fire(1)
	mock.Fire(2)
	mock.NothingFunc.SetDefaultReturn()
	mock.Nothing()
	t.DeepEqual(fired, []int{2})

	fireCalls := mock.FireFunc.History()
	t.Len(fireCalls, 2)
	t.DeepEqual(fireCalls[0].Args(), []any{1})
	t.DeepEqual(fireCalls[1].Results(), []any{})
	nothingCalls := mock.NothingFunc.History()
	t.Len(nothingCalls, 1)
	t.DeepEqual(nothingCalls[0].Args(), []any{})
	t.DeepEqual(nothingCalls[0].Results(), []any{})
}

func TestCollide(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	mock := gomockgentest.NewMockCollide()
	mock.NamesFunc.PushReturn(7, errTest)
	var mutex sync.Mutex
	r1, err := mock.Names(1, 2, 3, errTest, "v0", &mutex, []int{4}, true, "i")
	t.Equal(r1, 7)
	t.Err(err, errTest)
	t.DeepEqual(mock.NamesFunc.History()[0].Args(),
		[]any{1, 2, 3, errTest, "v0", &mutex, []int{4}, true, "i"})
}

func TestSwapper(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	mock := gomockgentest.NewMockSwapper()
	mock.SwapFunc.SetDefaultReturn(b.Thing{})
	t.Equal(mock.Swap(a.Thing{}), b.Thing{})
	t.Equal(mock.SwapFunc.History()[0].Arg0, a.Thing{})
}

func TestReadCloser(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	mock := gomockgentest.NewMockReadCloser()
	mock.ReadFunc.PushReturn(3, nil)
	mock.CloseFunc.SetDefaultReturn(errTest)
	n, err := mock.Read(make([]byte, 4))
	t.Equal(n, 3)
	t.Nil(err)
	t.Err(mock.Close(), errTest)
}

func TestGenerics(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	i2 := gomockgentest.NewMockI2[int, string]()
	i2.M2Func.PushReturn("two")
	i2.M1(1)
	t.Equal(i2.M2(2), "two")
	t.Equal(i2.M2(3), "")
	t.Equal(i2.M1Func.History()[0].Arg0, 1)
	t.Equal(i2.M2Func.History()[1].Arg0, 3)

	i2From := gomockgentest.NewMockI2From[int, string](i2)
	t.Equal(i2From.M2(4), "")
	t.Len(i2.M2Func.History(), 3)

	cmp := gomockgentest.NewMockCmp[string]()
	cmp.EqFunc.SetDefaultHook(func(x, y string) bool { return x == y })
	t.True(cmp.Eq("a", "a"))
	t.False(cmp.Eq("a", "b"))
	t.DeepEqual(cmp.EqFunc.History()[1].Args(), []any{"a", "b"})

	i3 := gomockgentest.NewStrictMockI3[int]()
	t.PanicMatch(func() { _ = i3.M3(nil) }, `^unexpected invocation of MockI3.M3$`)
	i3.M3Func.SetDefaultReturn(3)
	t.Equal(i3.M3(gomockgentest.NewMockI1[int]()), 3)

	i4 := gomockgentest.NewMockI4[string]()
	t.Equal(i4.M4(nil), 0)
}

func TestEmpty(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt)

	t.NotNil(gomockgentest.NewMockEmpty())
	t.NotNil(gomockgentest.NewStrictMockEmpty())
	t.NotNil(gomockgentest.NewMockEmptyFrom(nil))
}

func TestConcurrent(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt).MustAll()

	const workers = 8
	const calls = 100
	mock := gomockgentest.NewMockRepo()
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range calls {
				mock.PutFunc.PushReturn(errTest)
			}
		})
	}
	wg.Wait()
	for range workers {
		wg.Go(func() {
			for range calls {
				_ = mock.Put(t.Context(), "")
				_ = mock.PutFunc.History()
			}
		})
	}
	wg.Wait()

	history := mock.PutFunc.History()
	t.Len(history, workers*calls)
	for _, call := range history {
		t.Err(call.Result0, errTest)
	}
	t.Nil(mock.Put(t.Context(), ""))
}
