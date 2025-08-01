// Copyright 2017 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package java

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/blueprint"
	"github.com/google/blueprint/proptools"

	"android/soong/aconfig"
	"android/soong/android"
	"android/soong/cc"
	"android/soong/dexpreopt"
	"android/soong/genrule"
)

// Legacy preparer used for running tests within the java package.
//
// This includes everything that was needed to run any test in the java package prior to the
// introduction of the test fixtures. Tests that are being converted to use fixtures directly
// rather than through the testJava...() methods should avoid using this and instead use the
// various preparers directly, using android.GroupFixturePreparers(...) to group them when
// necessary.
//
// deprecated
var prepareForJavaTest = android.GroupFixturePreparers(
	genrule.PrepareForTestWithGenRuleBuildComponents,
	// Get the CC build components but not default modules.
	cc.PrepareForTestWithCcBuildComponents,
	// Include all the default java modules.
	PrepareForTestWithDexpreopt,
	// Include aconfig modules.
	aconfig.PrepareForTestWithAconfigBuildComponents,
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// testJavaError is a legacy way of running tests of java modules that expect errors.
//
// See testJava for an explanation as to how to stop using this deprecated method.
//
// deprecated
func testJavaError(t *testing.T, pattern string, bp string) (*android.TestContext, android.Config) {
	t.Helper()
	result := android.GroupFixturePreparers(
		prepareForJavaTest, dexpreopt.PrepareForTestByEnablingDexpreopt).
		ExtendWithErrorHandler(android.FixtureExpectsAtLeastOneErrorMatchingPattern(pattern)).
		RunTestWithBp(t, bp)
	return result.TestContext, result.Config
}

// testJavaWithFS runs tests using the prepareForJavaTest
//
// See testJava for an explanation as to how to stop using this deprecated method.
//
// deprecated
func testJavaWithFS(t *testing.T, bp string, fs android.MockFS) (*android.TestContext, android.Config) {
	t.Helper()
	result := android.GroupFixturePreparers(
		prepareForJavaTest, fs.AddToFixture()).RunTestWithBp(t, bp)
	return result.TestContext, result.Config
}

// testJava runs tests using the prepareForJavaTest
//
// Do not add any new usages of this, instead use the prepareForJavaTest directly as it makes it
// much easier to customize the test behavior.
//
// If it is necessary to customize the behavior of an existing test that uses this then please first
// convert the test to using prepareForJavaTest first and then in a following change add the
// appropriate fixture preparers. Keeping the conversion change separate makes it easy to verify
// that it did not change the test behavior unexpectedly.
//
// deprecated
func testJava(t *testing.T, bp string) (*android.TestContext, android.Config) {
	t.Helper()
	result := prepareForJavaTest.RunTestWithBp(t, bp)
	return result.TestContext, result.Config
}

// defaultModuleToPath constructs a path to the turbine generate jar for a default test module that
// is defined in PrepareForIntegrationTestWithJava
func defaultModuleToPath(name string) string {
	switch {
	case name == `""`:
		return name
	case strings.HasSuffix(name, ".jar"):
		return name
	default:
		return filepath.Join("out", "soong", ".intermediates", defaultJavaDir, name, "android_common", "turbine-combined", name+".jar")
	}
}

// Test that the PrepareForTestWithJavaDefaultModules provides all the files that it uses by
// running it in a fixture that requires all source files to exist.
func TestPrepareForTestWithJavaDefaultModules(t *testing.T) {
	android.GroupFixturePreparers(
		PrepareForTestWithJavaDefaultModules,
		android.PrepareForTestDisallowNonExistentPaths,
	).RunTest(t)
}

func TestJavaLinkType(t *testing.T) {
	testJava(t, `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			libs: ["bar"],
			static_libs: ["baz"],
		}

		java_library {
			name: "bar",
			sdk_version: "current",
			srcs: ["b.java"],
		}

		java_library {
			name: "baz",
			sdk_version: "system_current",
			srcs: ["c.java"],
		}
	`)

	testJavaError(t, "consider adjusting sdk_version: OR platform_apis:", `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			libs: ["bar"],
			sdk_version: "current",
			static_libs: ["baz"],
		}

		java_library {
			name: "bar",
			sdk_version: "current",
			srcs: ["b.java"],
		}

		java_library {
			name: "baz",
			sdk_version: "system_current",
			srcs: ["c.java"],
		}
	`)

	testJava(t, `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			libs: ["bar"],
			sdk_version: "system_current",
			static_libs: ["baz"],
		}

		java_library {
			name: "bar",
			sdk_version: "current",
			srcs: ["b.java"],
		}

		java_library {
			name: "baz",
			sdk_version: "system_current",
			srcs: ["c.java"],
		}
	`)

	testJavaError(t, "consider adjusting sdk_version: OR platform_apis:", `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			libs: ["bar"],
			sdk_version: "system_current",
			static_libs: ["baz"],
		}

		java_library {
			name: "bar",
			sdk_version: "current",
			srcs: ["b.java"],
		}

		java_library {
			name: "baz",
			srcs: ["c.java"],
		}
	`)
}

func TestSimple(t *testing.T) {
	bp := `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			libs: ["bar"],
			static_libs: ["baz"],
		}

		java_library {
			name: "bar",
			srcs: ["b.java"],
			static_libs: ["quz"],
		}

		java_library {
			name: "baz",
			srcs: ["c.java"],
			static_libs: ["quz"],
		}

		java_library {
			name: "quz",
			srcs: ["d.java"],
		}`

	frameworkTurbineCombinedJars := []string{
		"out/soong/.intermediates/default/java/ext/android_common/turbine-combined/ext.jar",
		"out/soong/.intermediates/default/java/framework/android_common/turbine-combined/framework.jar",
	}

	frameworkTurbineJars := []string{
		"out/soong/.intermediates/default/java/ext/android_common/turbine/ext.jar",
		"out/soong/.intermediates/default/java/framework/android_common/turbine/framework.jar",
	}

	testCases := []struct {
		name string

		preparer android.FixturePreparer

		fooJavacInputs          []string
		fooJavacClasspath       []string
		fooCombinedInputs       []string
		fooHeaderCombinedInputs []string

		barJavacInputs          []string
		barJavacClasspath       []string
		barCombinedInputs       []string
		barHeaderCombinedInputs []string
	}{
		{
			name:           "normal",
			preparer:       android.NullFixturePreparer,
			fooJavacInputs: []string{"a.java"},
			fooJavacClasspath: slices.Concat(
				frameworkTurbineCombinedJars,
				[]string{
					"out/soong/.intermediates/bar/android_common/turbine-combined/bar.jar",
					"out/soong/.intermediates/baz/android_common/turbine-combined/baz.jar",
				},
			),
			fooCombinedInputs: []string{
				"out/soong/.intermediates/foo/android_common/javac/foo.jar",
				"out/soong/.intermediates/baz/android_common/combined/baz.jar",
			},

			fooHeaderCombinedInputs: []string{
				"out/soong/.intermediates/foo/android_common/turbine/foo.jar",
				"out/soong/.intermediates/baz/android_common/turbine-combined/baz.jar",
			},

			barJavacInputs: []string{"b.java"},
			barJavacClasspath: slices.Concat(
				frameworkTurbineCombinedJars,
				[]string{
					"out/soong/.intermediates/quz/android_common/turbine-combined/quz.jar",
				},
			),
			barCombinedInputs: []string{
				"out/soong/.intermediates/bar/android_common/javac/bar.jar",
				"out/soong/.intermediates/quz/android_common/javac/quz.jar",
			},
			barHeaderCombinedInputs: []string{
				"out/soong/.intermediates/bar/android_common/turbine/bar.jar",
				"out/soong/.intermediates/quz/android_common/turbine-combined/quz.jar",
			},
		},
		{
			name:           "transitive classpath",
			preparer:       PrepareForTestWithTransitiveClasspathEnabled,
			fooJavacInputs: []string{"a.java"},
			fooJavacClasspath: slices.Concat(
				frameworkTurbineJars,
				[]string{
					"out/soong/.intermediates/bar/android_common/turbine/bar.jar",
					"out/soong/.intermediates/quz/android_common/turbine/quz.jar",
					"out/soong/.intermediates/baz/android_common/turbine/baz.jar",
				},
			),
			fooCombinedInputs: []string{
				"out/soong/.intermediates/foo/android_common/javac/foo.jar",
				"out/soong/.intermediates/baz/android_common/javac/baz.jar",
				"out/soong/.intermediates/quz/android_common/javac/quz.jar",
			},

			fooHeaderCombinedInputs: []string{
				"out/soong/.intermediates/foo/android_common/turbine/foo.jar",
				"out/soong/.intermediates/baz/android_common/turbine/baz.jar",
				"out/soong/.intermediates/quz/android_common/turbine/quz.jar",
			},

			barJavacInputs: []string{"b.java"},
			barJavacClasspath: slices.Concat(
				frameworkTurbineJars,
				[]string{"out/soong/.intermediates/quz/android_common/turbine/quz.jar"},
			),
			barCombinedInputs: []string{
				"out/soong/.intermediates/bar/android_common/javac/bar.jar",
				"out/soong/.intermediates/quz/android_common/javac/quz.jar",
			},
			barHeaderCombinedInputs: []string{
				"out/soong/.intermediates/bar/android_common/turbine/bar.jar",
				"out/soong/.intermediates/quz/android_common/turbine/quz.jar",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result := android.GroupFixturePreparers(
				PrepareForTestWithJavaDefaultModules,
				tt.preparer,
			).RunTestWithBp(t, bp)
			foo := result.ModuleForTests("foo", "android_common")

			fooJavac := foo.Rule("javac")
			android.AssertPathsRelativeToTopEquals(t, "foo javac inputs", tt.fooJavacInputs, fooJavac.Inputs)

			fooJavacClasspath := fooJavac.Args["classpath"]
			android.AssertStringPathsRelativeToTopEquals(t, "foo javac classpath", result.Config, tt.fooJavacClasspath,
				strings.Split(strings.TrimPrefix(fooJavacClasspath, "-classpath "), ":"))

			fooCombinedJar := foo.Output("combined/foo.jar")
			android.AssertPathsRelativeToTopEquals(t, "foo combined inputs", tt.fooCombinedInputs, fooCombinedJar.Inputs)

			fooCombinedHeaderJar := foo.Output("turbine-combined/foo.jar")
			android.AssertPathsRelativeToTopEquals(t, "foo header combined inputs", tt.fooHeaderCombinedInputs, fooCombinedHeaderJar.Inputs)

			bar := result.ModuleForTests("bar", "android_common")
			barJavac := bar.Rule("javac")
			android.AssertPathsRelativeToTopEquals(t, "bar javac inputs", tt.barJavacInputs, barJavac.Inputs)

			barJavacClasspath := barJavac.Args["classpath"]
			android.AssertStringPathsRelativeToTopEquals(t, "bar javac classpath", result.Config, tt.barJavacClasspath,
				strings.Split(strings.TrimPrefix(barJavacClasspath, "-classpath "), ":"))

			barCombinedJar := bar.Output("combined/bar.jar")
			android.AssertPathsRelativeToTopEquals(t, "bar combined inputs", tt.barCombinedInputs, barCombinedJar.Inputs)

			barCombinedHeaderJar := bar.Output("turbine-combined/bar.jar")
			android.AssertPathsRelativeToTopEquals(t, "bar header combined inputs", tt.barHeaderCombinedInputs, barCombinedHeaderJar.Inputs)
		})
	}
}

func TestExportedPlugins(t *testing.T) {
	type Result struct {
		library        string
		processors     string
		disableTurbine bool
	}
	var tests = []struct {
		name    string
		extra   string
		results []Result
	}{
		{
			name:    "Exported plugin is not a direct plugin",
			extra:   `java_library { name: "exports", srcs: ["a.java"], exported_plugins: ["plugin"] }`,
			results: []Result{{library: "exports", processors: "-proc:none"}},
		},
		{
			name: "Exports plugin to dependee",
			extra: `
				java_library{name: "exports", exported_plugins: ["plugin"]}
				java_library{name: "foo", srcs: ["a.java"], libs: ["exports"]}
				java_library{name: "bar", srcs: ["a.java"], static_libs: ["exports"]}
			`,
			results: []Result{
				{library: "foo", processors: "-processor com.android.TestPlugin"},
				{library: "bar", processors: "-processor com.android.TestPlugin"},
			},
		},
		{
			name: "Exports plugin to android_library",
			extra: `
				java_library{name: "exports", exported_plugins: ["plugin"]}
				android_library{name: "foo", srcs: ["a.java"],  libs: ["exports"]}
				android_library{name: "bar", srcs: ["a.java"], static_libs: ["exports"]}
			`,
			results: []Result{
				{library: "foo", processors: "-processor com.android.TestPlugin"},
				{library: "bar", processors: "-processor com.android.TestPlugin"},
			},
		},
		{
			name: "Exports plugin is not propagated via transitive deps",
			extra: `
				java_library{name: "exports", exported_plugins: ["plugin"]}
				java_library{name: "foo", srcs: ["a.java"], libs: ["exports"]}
				java_library{name: "bar", srcs: ["a.java"], static_libs: ["foo"]}
			`,
			results: []Result{
				{library: "foo", processors: "-processor com.android.TestPlugin"},
				{library: "bar", processors: "-proc:none"},
			},
		},
		{
			name: "Exports plugin appends to plugins",
			extra: `
                java_plugin{name: "plugin2", processor_class: "com.android.TestPlugin2"}
				java_library{name: "exports", exported_plugins: ["plugin"]}
				java_library{name: "foo", srcs: ["a.java"], libs: ["exports"], plugins: ["plugin2"]}
			`,
			results: []Result{
				{library: "foo", processors: "-processor com.android.TestPlugin,com.android.TestPlugin2"},
			},
		},
		{
			name: "Exports plugin to with generates_api to dependee",
			extra: `
				java_library{name: "exports", exported_plugins: ["plugin_generates_api"]}
				java_library{name: "foo", srcs: ["a.java"], libs: ["exports"]}
				java_library{name: "bar", srcs: ["a.java"], static_libs: ["exports"]}
			`,
			results: []Result{
				{library: "foo", processors: "-processor com.android.TestPlugin", disableTurbine: true},
				{library: "bar", processors: "-processor com.android.TestPlugin", disableTurbine: true},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := testJava(t, `
				java_plugin {
					name: "plugin",
					processor_class: "com.android.TestPlugin",
				}
				java_plugin {
					name: "plugin_generates_api",
					generates_api: true,
					processor_class: "com.android.TestPlugin",
				}
			`+test.extra)

			for _, want := range test.results {
				javac := ctx.ModuleForTests(want.library, "android_common").Rule("javac")
				if javac.Args["processor"] != want.processors {
					t.Errorf("For library %v, expected %v, found %v", want.library, want.processors, javac.Args["processor"])
				}
				turbine := ctx.ModuleForTests(want.library, "android_common").MaybeRule("turbine")
				disableTurbine := turbine.BuildParams.Rule == nil
				if disableTurbine != want.disableTurbine {
					t.Errorf("For library %v, expected disableTurbine %v, found %v", want.library, want.disableTurbine, disableTurbine)
				}
			}
		})
	}
}

func TestSdkVersionByPartition(t *testing.T) {
	testJavaError(t, "sdk_version must have a value when the module is located at vendor or product", `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			vendor: true,
		}
	`)

	testJava(t, `
		java_library {
			name: "bar",
			srcs: ["b.java"],
		}
	`)

	for _, enforce := range []bool{true, false} {
		bp := `
			java_library {
				name: "foo",
				srcs: ["a.java"],
				product_specific: true,
			}
		`

		errorHandler := android.FixtureExpectsNoErrors
		if enforce {
			errorHandler = android.FixtureExpectsAtLeastOneErrorMatchingPattern("sdk_version must have a value when the module is located at vendor or product")
		}

		android.GroupFixturePreparers(
			PrepareForTestWithJavaDefaultModules,
			android.FixtureModifyProductVariables(func(variables android.FixtureProductVariables) {
				variables.EnforceProductPartitionInterface = proptools.BoolPtr(enforce)
			}),
		).
			ExtendWithErrorHandler(errorHandler).
			RunTestWithBp(t, bp)
	}
}

func TestArchSpecific(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			target: {
				android: {
					srcs: ["b.java"],
				},
			},
		}
	`)

	javac := ctx.ModuleForTests("foo", "android_common").Rule("javac")
	if len(javac.Inputs) != 2 || javac.Inputs[0].String() != "a.java" || javac.Inputs[1].String() != "b.java" {
		t.Errorf(`foo inputs %v != ["a.java", "b.java"]`, javac.Inputs)
	}
}

func TestBinary(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library_host {
			name: "foo",
			srcs: ["a.java"],
		}

		java_binary_host {
			name: "bar",
			srcs: ["b.java"],
			static_libs: ["foo"],
			jni_libs: ["libjni"],
		}

		cc_library_shared {
			name: "libjni",
			host_supported: true,
			device_supported: false,
			stl: "none",
		}
	`)

	buildOS := ctx.Config().BuildOS.String()

	bar := ctx.ModuleForTests("bar", buildOS+"_common")
	barJar := bar.Output("bar.jar").Output.String()
	barWrapperDeps := bar.Output("bar").Implicits.Strings()

	libjni := ctx.ModuleForTests("libjni", buildOS+"_x86_64_shared")
	libjniSO := libjni.Rule("Cp").Output.String()

	// Test that the install binary wrapper depends on the installed jar file
	if g, w := barWrapperDeps, barJar; !android.InList(w, g) {
		t.Errorf("expected binary wrapper implicits to contain %q, got %q", w, g)
	}

	// Test that the install binary wrapper depends on the installed JNI libraries
	if g, w := barWrapperDeps, libjniSO; !android.InList(w, g) {
		t.Errorf("expected binary wrapper implicits to contain %q, got %q", w, g)
	}
}

func TestTest(t *testing.T) {
	ctx, _ := testJava(t, `
		java_test_host {
			name: "foo",
			srcs: ["a.java"],
			jni_libs: ["libjni"],
		}

		cc_library_shared {
			name: "libjni",
			host_supported: true,
			device_supported: false,
			stl: "none",
		}
	`)

	buildOS := ctx.Config().BuildOS.String()

	foo := ctx.ModuleForTests("foo", buildOS+"_common").Module().(*TestHost)

	expected := "lib64/libjni.so"
	if runtime.GOOS == "darwin" {
		expected = "lib64/libjni.dylib"
	}

	fooTestData := foo.data
	if len(fooTestData) != 1 || fooTestData[0].Rel() != expected {
		t.Errorf(`expected foo test data relative path [%q], got %q`,
			expected, fooTestData.Strings())
	}
}

func TestHostBinaryNoJavaDebugInfoOverride(t *testing.T) {
	bp := `
		java_library {
			name: "target_library",
			srcs: ["a.java"],
		}

		java_binary_host {
			name: "host_binary",
			srcs: ["b.java"],
		}
	`

	result := android.GroupFixturePreparers(
		PrepareForTestWithJavaDefaultModules,
		android.FixtureModifyProductVariables(func(variables android.FixtureProductVariables) {
			variables.MinimizeJavaDebugInfo = proptools.BoolPtr(true)
		}),
	).RunTestWithBp(t, bp)

	// first, check that the -g flag is added to target modules
	targetLibrary := result.ModuleForTests("target_library", "android_common")
	targetJavaFlags := targetLibrary.Module().VariablesForTests()["javacFlags"]
	if !strings.Contains(targetJavaFlags, "-g:source,lines") {
		t.Errorf("target library javac flags %v should contain "+
			"-g:source,lines override with MinimizeJavaDebugInfo", targetJavaFlags)
	}

	// check that -g is not overridden for host modules
	buildOS := result.Config.BuildOS.String()
	hostBinary := result.ModuleForTests("host_binary", buildOS+"_common")
	hostJavaFlags := hostBinary.Module().VariablesForTests()["javacFlags"]
	if strings.Contains(hostJavaFlags, "-g:source,lines") {
		t.Errorf("java_binary_host javac flags %v should not have "+
			"-g:source,lines override with MinimizeJavaDebugInfo", hostJavaFlags)
	}
}

// A minimal context object for use with DexJarBuildPath
type moduleErrorfTestCtx struct {
}

func (ctx moduleErrorfTestCtx) ModuleErrorf(format string, args ...interface{}) {
}

var _ android.ModuleErrorfContext = (*moduleErrorfTestCtx)(nil)

func TestPrebuilts(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["a.java", ":stubs-source"],
			libs: ["bar", "sdklib.stubs"],
			static_libs: ["baz"],
		}

		java_import {
			name: "bar",
			jars: ["a.jar"],
		}

		java_import {
			name: "baz",
			jars: ["b.jar"],
			sdk_version: "current",
			compile_dex: true,
		}

		dex_import {
			name: "qux",
			jars: ["b.jar"],
		}

		java_sdk_library_import {
			name: "sdklib",
			public: {
				jars: ["c.jar"],
			},
		}

		prebuilt_stubs_sources {
			name: "stubs-source",
			srcs: ["stubs/sources"],
		}

		java_test_import {
			name: "test",
			jars: ["a.jar"],
			test_suites: ["cts"],
			test_config: "AndroidTest.xml",
		}
		`)

	fooModule := ctx.ModuleForTests("foo", "android_common")
	javac := fooModule.Rule("javac")
	combineJar := ctx.ModuleForTests("foo", "android_common").Description("for javac")
	barModule := ctx.ModuleForTests("bar", "android_common")
	barJar := barModule.Output("combined/bar.jar").Output
	bazModule := ctx.ModuleForTests("baz", "android_common")
	bazJar := bazModule.Output("combined/baz.jar").Output
	sdklibStubsJar := ctx.ModuleForTests("sdklib.stubs", "android_common").
		Output("combined/sdklib.stubs.jar").Output

	fooLibrary := fooModule.Module().(*Library)
	assertDeepEquals(t, "foo unique sources incorrect",
		[]string{"a.java"}, fooLibrary.uniqueSrcFiles.Strings())

	assertDeepEquals(t, "foo java source jars incorrect",
		[]string{".intermediates/stubs-source/android_common/stubs-source-stubs.srcjar"},
		android.NormalizePathsForTesting(fooLibrary.compiledSrcJars))

	if !strings.Contains(javac.Args["classpath"], barJar.String()) {
		t.Errorf("foo classpath %v does not contain %q", javac.Args["classpath"], barJar.String())
	}

	errCtx := moduleErrorfTestCtx{}
	barDexJar := barModule.Module().(*Import).DexJarBuildPath(errCtx)
	if barDexJar.IsSet() {
		t.Errorf("bar dex jar build path expected to be set, got %s", barDexJar)
	}

	if !strings.Contains(javac.Args["classpath"], sdklibStubsJar.String()) {
		t.Errorf("foo classpath %v does not contain %q", javac.Args["classpath"], sdklibStubsJar.String())
	}

	if len(combineJar.Inputs) != 2 || combineJar.Inputs[1].String() != bazJar.String() {
		t.Errorf("foo combineJar inputs %v does not contain %q", combineJar.Inputs, bazJar.String())
	}

	bazDexJar := bazModule.Module().(*Import).DexJarBuildPath(errCtx).Path()
	expectedDexJar := "out/soong/.intermediates/baz/android_common/dex/baz.jar"
	android.AssertPathRelativeToTopEquals(t, "baz dex jar build path", expectedDexJar, bazDexJar)

	ctx.ModuleForTests("qux", "android_common").Rule("Cp")

	entries := android.AndroidMkEntriesForTest(t, ctx, fooModule.Module())[0]
	android.AssertStringEquals(t, "unexpected LOCAL_SOONG_MODULE_TYPE", "java_library", entries.EntryMap["LOCAL_SOONG_MODULE_TYPE"][0])
	entries = android.AndroidMkEntriesForTest(t, ctx, barModule.Module())[0]
	android.AssertStringEquals(t, "unexpected LOCAL_SOONG_MODULE_TYPE", "java_import", entries.EntryMap["LOCAL_SOONG_MODULE_TYPE"][0])
}

func assertDeepEquals(t *testing.T, message string, expected interface{}, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%s: expected %q, found %q", message, expected, actual)
	}
}

func TestPrebuiltStubsSources(t *testing.T) {
	test := func(t *testing.T, sourcesPath string, expectedInputs []string) {
		ctx, _ := testJavaWithFS(t, fmt.Sprintf(`
prebuilt_stubs_sources {
  name: "stubs-source",
	srcs: ["%s"],
}`, sourcesPath), map[string][]byte{
			"stubs/sources/pkg/A.java": nil,
			"stubs/sources/pkg/B.java": nil,
		})

		zipSrc := ctx.ModuleForTests("stubs-source", "android_common").Rule("zip_src")
		if expected, actual := expectedInputs, zipSrc.Inputs.Strings(); !reflect.DeepEqual(expected, actual) {
			t.Errorf("mismatch of inputs to soong_zip: expected %q, actual %q", expected, actual)
		}
	}

	t.Run("empty/missing directory", func(t *testing.T) {
		test(t, "empty-directory", nil)
	})

	t.Run("non-empty set of sources", func(t *testing.T) {
		test(t, "stubs/sources", []string{
			"stubs/sources/pkg/A.java",
			"stubs/sources/pkg/B.java",
		})
	})
}

func TestDefaults(t *testing.T) {
	ctx, _ := testJava(t, `
		java_defaults {
			name: "defaults",
			srcs: ["a.java"],
			libs: ["bar"],
			static_libs: ["baz"],
			optimize: {enabled: false},
		}

		java_library {
			name: "foo",
			defaults: ["defaults"],
		}

		java_library {
			name: "bar",
			srcs: ["b.java"],
		}

		java_library {
			name: "baz",
			srcs: ["c.java"],
		}

		android_test {
			name: "atestOptimize",
			defaults: ["defaults"],
			optimize: {enabled: true},
		}

		android_test {
			name: "atestNoOptimize",
			defaults: ["defaults"],
		}

		android_test {
			name: "atestDefault",
			srcs: ["a.java"],
		}
		`)

	javac := ctx.ModuleForTests("foo", "android_common").Rule("javac")
	combineJar := ctx.ModuleForTests("foo", "android_common").Description("for javac")

	if len(javac.Inputs) != 1 || javac.Inputs[0].String() != "a.java" {
		t.Errorf(`foo inputs %v != ["a.java"]`, javac.Inputs)
	}

	barTurbine := filepath.Join("out", "soong", ".intermediates", "bar", "android_common", "turbine-combined", "bar.jar")
	if !strings.Contains(javac.Args["classpath"], barTurbine) {
		t.Errorf("foo classpath %v does not contain %q", javac.Args["classpath"], barTurbine)
	}

	baz := ctx.ModuleForTests("baz", "android_common").Rule("javac").Output.String()
	if len(combineJar.Inputs) != 2 || combineJar.Inputs[1].String() != baz {
		t.Errorf("foo combineJar inputs %v does not contain %q", combineJar.Inputs, baz)
	}

	atestOptimize := ctx.ModuleForTests("atestOptimize", "android_common").MaybeRule("r8")
	if atestOptimize.Output == nil {
		t.Errorf("atestOptimize should optimize APK")
	}

	atestNoOptimize := ctx.ModuleForTests("atestNoOptimize", "android_common").MaybeRule("d8")
	if atestNoOptimize.Output == nil {
		t.Errorf("atestNoOptimize should not optimize APK")
	}

	atestDefault := ctx.ModuleForTests("atestDefault", "android_common").MaybeRule("d8")
	if atestDefault.Output == nil {
		t.Errorf("atestDefault should not optimize APK")
	}
}

func TestResources(t *testing.T) {
	var table = []struct {
		name  string
		prop  string
		extra string
		args  string
	}{
		{
			// Test that a module with java_resource_dirs includes the files
			name: "resource dirs",
			prop: `java_resource_dirs: ["java-res"]`,
			args: "-C java-res -f java-res/a/a -f java-res/b/b",
		},
		{
			// Test that a module with java_resources includes the files
			name: "resource files",
			prop: `java_resources: ["java-res/a/a", "java-res/b/b"]`,
			args: "-C . -f java-res/a/a -f java-res/b/b",
		},
		{
			// Test that a module with a filegroup in java_resources includes the files with the
			// path prefix
			name: "resource filegroup",
			prop: `java_resources: [":foo-res"]`,
			extra: `
				filegroup {
					name: "foo-res",
					path: "java-res",
					srcs: ["java-res/a/a", "java-res/b/b"],
				}`,
			args: "-C java-res -f java-res/a/a -f java-res/b/b",
		},
		{
			// Test that a module with wildcards in java_resource_dirs has the correct path prefixes
			name: "wildcard dirs",
			prop: `java_resource_dirs: ["java-res/*"]`,
			args: "-C java-res/a -f java-res/a/a -C java-res/b -f java-res/b/b",
		},
		{
			// Test that a module exclude_java_resource_dirs excludes the files
			name: "wildcard dirs",
			prop: `java_resource_dirs: ["java-res/*"], exclude_java_resource_dirs: ["java-res/b"]`,
			args: "-C java-res/a -f java-res/a/a",
		},
		{
			// Test wildcards in java_resources
			name: "wildcard files",
			prop: `java_resources: ["java-res/**/*"]`,
			args: "-C . -f java-res/a/a -f java-res/b/b",
		},
		{
			// Test exclude_java_resources with java_resources
			name: "wildcard files with exclude",
			prop: `java_resources: ["java-res/**/*"], exclude_java_resources: ["java-res/b/*"]`,
			args: "-C . -f java-res/a/a",
		},
		{
			// Test exclude_java_resources with java_resource_dirs
			name: "resource dirs with exclude files",
			prop: `java_resource_dirs: ["java-res"], exclude_java_resources: ["java-res/b/b"]`,
			args: "-C java-res -f java-res/a/a",
		},
		{
			// Test exclude_java_resource_dirs with java_resource_dirs
			name: "resource dirs with exclude files",
			prop: `java_resource_dirs: ["java-res", "java-res2"], exclude_java_resource_dirs: ["java-res2"]`,
			args: "-C java-res -f java-res/a/a -f java-res/b/b",
		},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := testJavaWithFS(t, `
				java_library {
					name: "foo",
					srcs: [
						"a.java",
						"b.java",
						"c.java",
					],
					`+test.prop+`,
				}
			`+test.extra,
				map[string][]byte{
					"java-res/a/a": nil,
					"java-res/b/b": nil,
					"java-res2/a":  nil,
				},
			)

			foo := ctx.ModuleForTests("foo", "android_common").Output("withres/foo.jar")
			fooRes := ctx.ModuleForTests("foo", "android_common").Output("res/foo.jar")

			if !inList(fooRes.Output.String(), foo.Inputs.Strings()) {
				t.Errorf("foo combined jars %v does not contain %q",
					foo.Inputs.Strings(), fooRes.Output.String())
			}

			if fooRes.Args["jarArgs"] != test.args {
				t.Errorf("foo resource jar args %q is not %q",
					fooRes.Args["jarArgs"], test.args)
			}
		})
	}
}

func TestIncludeSrcs(t *testing.T) {
	ctx, _ := testJavaWithFS(t, `
		java_library {
			name: "foo",
			srcs: [
				"a.java",
				"b.java",
				"c.java",
			],
			include_srcs: true,
		}

		java_library {
			name: "bar",
			srcs: [
				"a.java",
				"b.java",
				"c.java",
			],
			java_resource_dirs: ["java-res"],
			include_srcs: true,
		}
	`, map[string][]byte{
		"java-res/a/a": nil,
		"java-res/b/b": nil,
		"java-res2/a":  nil,
	})

	// Test a library with include_srcs: true
	foo := ctx.ModuleForTests("foo", "android_common").Output("withres/foo.jar")
	fooSrcJar := ctx.ModuleForTests("foo", "android_common").Output("foo.srcjar")

	if g, w := fooSrcJar.Output.String(), foo.Inputs.Strings(); !inList(g, w) {
		t.Errorf("foo combined jars %v does not contain %q", w, g)
	}

	if g, w := fooSrcJar.Args["jarArgs"], "-C . -f a.java -f b.java -f c.java"; g != w {
		t.Errorf("foo source jar args %q is not %q", w, g)
	}

	// Test a library with include_srcs: true and resources
	bar := ctx.ModuleForTests("bar", "android_common").Output("withres/bar.jar")
	barResCombined := ctx.ModuleForTests("bar", "android_common").Output("res-combined/bar.jar")
	barRes := ctx.ModuleForTests("bar", "android_common").Output("res/bar.jar")
	barSrcJar := ctx.ModuleForTests("bar", "android_common").Output("bar.srcjar")

	if g, w := barSrcJar.Output.String(), barResCombined.Inputs.Strings(); !inList(g, w) {
		t.Errorf("bar combined resource jars %v does not contain %q", w, g)
	}

	if g, w := barRes.Output.String(), barResCombined.Inputs.Strings(); !inList(g, w) {
		t.Errorf("bar combined resource jars %v does not contain %q", w, g)
	}

	if g, w := barResCombined.Output.String(), bar.Inputs.Strings(); !inList(g, w) {
		t.Errorf("bar combined jars %v does not contain %q", w, g)
	}

	if g, w := barSrcJar.Args["jarArgs"], "-C . -f a.java -f b.java -f c.java"; g != w {
		t.Errorf("bar source jar args %q is not %q", w, g)
	}

	if g, w := barRes.Args["jarArgs"], "-C java-res -f java-res/a/a -f java-res/b/b"; g != w {
		t.Errorf("bar resource jar args %q is not %q", w, g)
	}
}

func TestGeneratedSources(t *testing.T) {
	ctx, _ := testJavaWithFS(t, `
		java_library {
			name: "foo",
			srcs: [
				"a*.java",
				":gen",
				"b*.java",
			],
		}

		genrule {
			name: "gen",
			tool_files: ["java-res/a"],
			out: ["gen.java"],
		}
	`, map[string][]byte{
		"a.java": nil,
		"b.java": nil,
	})

	javac := ctx.ModuleForTests("foo", "android_common").Rule("javac")
	genrule := ctx.ModuleForTests("gen", "").Rule("generator")

	if filepath.Base(genrule.Output.String()) != "gen.java" {
		t.Fatalf(`gen output file %v is not ".../gen.java"`, genrule.Output.String())
	}

	if len(javac.Inputs) != 3 ||
		javac.Inputs[0].String() != "a.java" ||
		javac.Inputs[1].String() != genrule.Output.String() ||
		javac.Inputs[2].String() != "b.java" {
		t.Errorf(`foo inputs %v != ["a.java", ".../gen.java", "b.java"]`, javac.Inputs)
	}
}

func TestTurbine(t *testing.T) {
	result := android.GroupFixturePreparers(
		prepareForJavaTest, FixtureWithPrebuiltApis(map[string][]string{"14": {"foo"}})).
		RunTestWithBp(t, `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			sdk_version: "14",
		}

		java_library {
			name: "bar",
			srcs: ["b.java"],
			static_libs: ["foo"],
			sdk_version: "14",
		}

		java_library {
			name: "baz",
			srcs: ["c.java"],
			libs: ["bar"],
			sdk_version: "14",
		}
		`)

	fooTurbine := result.ModuleForTests("foo", "android_common").Rule("turbine")
	barTurbine := result.ModuleForTests("bar", "android_common").Rule("turbine")
	barJavac := result.ModuleForTests("bar", "android_common").Rule("javac")
	barTurbineCombined := result.ModuleForTests("bar", "android_common").Description("for turbine")
	bazJavac := result.ModuleForTests("baz", "android_common").Rule("javac")

	android.AssertPathsRelativeToTopEquals(t, "foo inputs", []string{"a.java"}, fooTurbine.Inputs)

	fooHeaderJar := filepath.Join("out", "soong", ".intermediates", "foo", "android_common", "turbine-combined", "foo.jar")
	barTurbineJar := filepath.Join("out", "soong", ".intermediates", "bar", "android_common", "turbine", "bar.jar")
	android.AssertStringDoesContain(t, "bar turbine classpath", barTurbine.Args["turbineFlags"], fooHeaderJar)
	android.AssertStringDoesContain(t, "bar javac classpath", barJavac.Args["classpath"], fooHeaderJar)
	android.AssertPathsRelativeToTopEquals(t, "bar turbine combineJar", []string{barTurbineJar, fooHeaderJar}, barTurbineCombined.Inputs)
	android.AssertStringDoesContain(t, "baz javac classpath", bazJavac.Args["classpath"], "prebuilts/sdk/14/public/android.jar")
}

func TestSharding(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "bar",
			srcs: ["a.java","b.java","c.java"],
			javac_shard_size: 1
		}
		`)

	barHeaderJar := filepath.Join("out", "soong", ".intermediates", "bar", "android_common", "turbine", "bar.jar")
	for i := 0; i < 3; i++ {
		barJavac := ctx.ModuleForTests("bar", "android_common").Description("javac" + strconv.Itoa(i))
		if !strings.HasPrefix(barJavac.Args["classpath"], "-classpath "+barHeaderJar+":") {
			t.Errorf("bar javac classpath %v does start with %q", barJavac.Args["classpath"], barHeaderJar)
		}
	}
}

func TestExcludeFileGroupInSrcs(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["a.java", ":foo-srcs"],
			exclude_srcs: ["a.java", ":foo-excludes"],
		}

		filegroup {
			name: "foo-srcs",
			srcs: ["java-fg/a.java", "java-fg/b.java", "java-fg/c.java"],
		}

		filegroup {
			name: "foo-excludes",
			srcs: ["java-fg/a.java", "java-fg/b.java"],
		}
	`)

	javac := ctx.ModuleForTests("foo", "android_common").Rule("javac")

	if len(javac.Inputs) != 1 || javac.Inputs[0].String() != "java-fg/c.java" {
		t.Errorf(`foo inputs %v != ["java-fg/c.java"]`, javac.Inputs)
	}
}

func TestJavaLibraryOutputFiles(t *testing.T) {
	testJavaWithFS(t, "", map[string][]byte{
		"libcore/Android.bp": []byte(`
				java_library {
						name: "core",
						sdk_version: "none",
						system_modules: "none",
				}

				filegroup {
					name: "core-jar",
					device_common_srcs: [":core{.jar}"],
				}
		`),
	})
}

func TestJavaImportOutputFiles(t *testing.T) {
	testJavaWithFS(t, "", map[string][]byte{
		"libcore/Android.bp": []byte(`
				java_import {
						name: "core",
						sdk_version: "none",
				}

				filegroup {
					name: "core-jar",
					device_common_srcs: [":core{.jar}"],
				}
		`),
	})
}

func TestJavaImport(t *testing.T) {
	bp := `
		java_library {
			name: "source_library",
			srcs: ["source.java"],
		}

		java_import {
			name: "import_with_no_deps",
			jars: ["no_deps.jar"],
		}

		java_import {
			name: "import_with_source_deps",
			jars: ["source_deps.jar"],
			static_libs: ["source_library"],
		}

		java_import {
			name: "import_with_import_deps",
			jars: ["import_deps.jar"],
			static_libs: ["import_with_no_deps"],
		}
	`
	ctx := android.GroupFixturePreparers(
		PrepareForTestWithJavaDefaultModules,
	).RunTestWithBp(t, bp)

	source := ctx.ModuleForTests("source_library", "android_common")
	sourceJar := source.Output("javac/source_library.jar")
	sourceHeaderJar := source.Output("turbine-combined/source_library.jar")
	sourceJavaInfo, _ := android.OtherModuleProvider(ctx, source.Module(), JavaInfoProvider)

	// The source library produces separate implementation and header jars
	android.AssertPathsRelativeToTopEquals(t, "source library implementation jar",
		[]string{sourceJar.Output.String()}, sourceJavaInfo.ImplementationAndResourcesJars)
	android.AssertPathsRelativeToTopEquals(t, "source library header jar",
		[]string{sourceHeaderJar.Output.String()}, sourceJavaInfo.HeaderJars)

	importWithNoDeps := ctx.ModuleForTests("import_with_no_deps", "android_common")
	importWithNoDepsJar := importWithNoDeps.Output("combined/import_with_no_deps.jar")
	importWithNoDepsJavaInfo, _ := android.OtherModuleProvider(ctx, importWithNoDeps.Module(), JavaInfoProvider)

	// An import with no deps produces a single jar used as both the header and implementation jar.
	android.AssertPathsRelativeToTopEquals(t, "import with no deps implementation jar",
		[]string{importWithNoDepsJar.Output.String()}, importWithNoDepsJavaInfo.ImplementationAndResourcesJars)
	android.AssertPathsRelativeToTopEquals(t, "import with no deps header jar",
		[]string{importWithNoDepsJar.Output.String()}, importWithNoDepsJavaInfo.HeaderJars)
	android.AssertPathsRelativeToTopEquals(t, "import with no deps combined inputs",
		[]string{"no_deps.jar"}, importWithNoDepsJar.Inputs)

	importWithSourceDeps := ctx.ModuleForTests("import_with_source_deps", "android_common")
	importWithSourceDepsJar := importWithSourceDeps.Output("combined/import_with_source_deps.jar")
	importWithSourceDepsHeaderJar := importWithSourceDeps.Output("turbine-combined/import_with_source_deps.jar")
	importWithSourceDepsJavaInfo, _ := android.OtherModuleProvider(ctx, importWithSourceDeps.Module(), JavaInfoProvider)

	// An import with source deps produces separate header and implementation jars.
	android.AssertPathsRelativeToTopEquals(t, "import with source deps implementation jar",
		[]string{importWithSourceDepsJar.Output.String()}, importWithSourceDepsJavaInfo.ImplementationAndResourcesJars)
	android.AssertPathsRelativeToTopEquals(t, "import with source deps header jar",
		[]string{importWithSourceDepsHeaderJar.Output.String()}, importWithSourceDepsJavaInfo.HeaderJars)
	android.AssertPathsRelativeToTopEquals(t, "import with source deps combined implementation jar inputs",
		[]string{"source_deps.jar", sourceJar.Output.String()}, importWithSourceDepsJar.Inputs)
	android.AssertPathsRelativeToTopEquals(t, "import with source deps combined header jar inputs",
		[]string{"source_deps.jar", sourceHeaderJar.Output.String()}, importWithSourceDepsHeaderJar.Inputs)

	importWithImportDeps := ctx.ModuleForTests("import_with_import_deps", "android_common")
	importWithImportDepsJar := importWithImportDeps.Output("combined/import_with_import_deps.jar")
	importWithImportDepsJavaInfo, _ := android.OtherModuleProvider(ctx, importWithImportDeps.Module(), JavaInfoProvider)

	// An import with only import deps produces a single jar used as both the header and implementation jar.
	android.AssertPathsRelativeToTopEquals(t, "import with import deps implementation jar",
		[]string{importWithImportDepsJar.Output.String()}, importWithImportDepsJavaInfo.ImplementationAndResourcesJars)
	android.AssertPathsRelativeToTopEquals(t, "import with import deps header jar",
		[]string{importWithImportDepsJar.Output.String()}, importWithImportDepsJavaInfo.HeaderJars)
	android.AssertPathsRelativeToTopEquals(t, "import with import deps combined implementation jar inputs",
		[]string{"import_deps.jar", importWithNoDepsJar.Output.String()}, importWithImportDepsJar.Inputs)
}

var compilerFlagsTestCases = []struct {
	in  string
	out bool
}{
	{
		in:  "a",
		out: false,
	},
	{
		in:  "-a",
		out: true,
	},
	{
		in:  "-no-jdk",
		out: false,
	},
	{
		in:  "-no-stdlib",
		out: false,
	},
	{
		in:  "-kotlin-home",
		out: false,
	},
	{
		in:  "-kotlin-home /some/path",
		out: false,
	},
	{
		in:  "-include-runtime",
		out: false,
	},
	{
		in:  "-Xintellij-plugin-root",
		out: false,
	},
}

type mockContext struct {
	android.ModuleContext
	result bool
}

func (ctx *mockContext) PropertyErrorf(property, format string, args ...interface{}) {
	// CheckBadCompilerFlags calls this function when the flag should be rejected
	ctx.result = false
}

func TestCompilerFlags(t *testing.T) {
	for _, testCase := range compilerFlagsTestCases {
		ctx := &mockContext{result: true}
		CheckKotlincFlags(ctx, []string{testCase.in})
		if ctx.result != testCase.out {
			t.Errorf("incorrect output:")
			t.Errorf("     input: %#v", testCase.in)
			t.Errorf("  expected: %#v", testCase.out)
			t.Errorf("       got: %#v", ctx.result)
		}
	}
}

// TODO(jungjw): Consider making this more robust by ignoring path order.
func checkPatchModuleFlag(t *testing.T, ctx *android.TestContext, moduleName string, expected string) {
	variables := ctx.ModuleForTests(moduleName, "android_common").VariablesForTestsRelativeToTop()
	flags := strings.Split(variables["javacFlags"], " ")
	got := ""
	for _, flag := range flags {
		keyEnd := strings.Index(flag, "=")
		if keyEnd > -1 && flag[:keyEnd] == "--patch-module" {
			got = flag[keyEnd+1:]
			break
		}
	}
	if expected != android.StringPathRelativeToTop(ctx.Config().SoongOutDir(), got) {
		t.Errorf("Unexpected patch-module flag for module %q - expected %q, but got %q", moduleName, expected, got)
	}
}

func TestPatchModule(t *testing.T) {
	t.Run("Java language level 8", func(t *testing.T) {
		// Test with legacy javac -source 1.8 -target 1.8
		bp := `
			java_library {
				name: "foo",
				srcs: ["a.java"],
				java_version: "1.8",
			}

			java_library {
				name: "bar",
				srcs: ["b.java"],
				sdk_version: "none",
				system_modules: "none",
				patch_module: "java.base",
				java_version: "1.8",
			}

			java_library {
				name: "baz",
				srcs: ["c.java"],
				patch_module: "java.base",
				java_version: "1.8",
			}
		`
		ctx, _ := testJava(t, bp)

		checkPatchModuleFlag(t, ctx, "foo", "")
		checkPatchModuleFlag(t, ctx, "bar", "")
		checkPatchModuleFlag(t, ctx, "baz", "")
	})

	t.Run("Java language level 9", func(t *testing.T) {
		// Test with default javac -source 9 -target 9
		bp := `
			java_library {
				name: "foo",
				srcs: ["a.java"],
			}

			java_library {
				name: "bar",
				srcs: ["b.java"],
				sdk_version: "none",
				system_modules: "none",
				patch_module: "java.base",
			}

			java_library {
				name: "baz",
				srcs: [
					"c.java",
					// Tests for b/150878007
					"dir/d.java",
					"dir2/e.java",
					"dir2/f.java",
					"nested/dir/g.java"
				],
				patch_module: "java.base",
			}
		`
		ctx, _ := testJava(t, bp)

		checkPatchModuleFlag(t, ctx, "foo", "")
		expected := "java.base=.:out/soong"
		checkPatchModuleFlag(t, ctx, "bar", expected)
		expected = "java.base=" + strings.Join([]string{
			".", "out/soong", defaultModuleToPath("ext"), defaultModuleToPath("framework")}, ":")
		checkPatchModuleFlag(t, ctx, "baz", expected)
	})
}

func TestJavaLibraryWithSystemModules(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
		    name: "lib-with-source-system-modules",
		    srcs: [
		        "a.java",
		    ],
				sdk_version: "none",
				system_modules: "source-system-modules",
		}

		java_library {
				name: "source-jar",
		    srcs: [
		        "a.java",
		    ],
		}

		java_system_modules {
				name: "source-system-modules",
				libs: ["source-jar"],
		}

		java_library {
		    name: "lib-with-prebuilt-system-modules",
		    srcs: [
		        "a.java",
		    ],
				sdk_version: "none",
				system_modules: "prebuilt-system-modules",
		}

		java_import {
				name: "prebuilt-jar",
				jars: ["a.jar"],
		}

		java_system_modules_import {
				name: "prebuilt-system-modules",
				libs: ["prebuilt-jar"],
		}
		`)

	checkBootClasspathForLibWithSystemModule(t, ctx, "lib-with-source-system-modules", "/source-jar.jar")

	checkBootClasspathForLibWithSystemModule(t, ctx, "lib-with-prebuilt-system-modules", "/prebuilt-jar.jar")
}

func checkBootClasspathForLibWithSystemModule(t *testing.T, ctx *android.TestContext, moduleName string, expectedSuffix string) {
	javacRule := ctx.ModuleForTests(moduleName, "android_common").Rule("javac")
	bootClasspath := javacRule.Args["bootClasspath"]
	if strings.HasPrefix(bootClasspath, "--system ") && strings.HasSuffix(bootClasspath, expectedSuffix) {
		t.Errorf("bootclasspath of %q must start with --system and end with %q, but was %#v.", moduleName, expectedSuffix, bootClasspath)
	}
}

func TestAidlExportIncludeDirsFromImports(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["aidl/foo/IFoo.aidl"],
			libs: ["bar"],
		}

		java_import {
			name: "bar",
			jars: ["a.jar"],
			aidl: {
				export_include_dirs: ["aidl/bar"],
			},
		}
	`)

	aidlCommand := ctx.ModuleForTests("foo", "android_common").Rule("aidl").RuleParams.Command
	expectedAidlFlag := "-Iaidl/bar"
	if !strings.Contains(aidlCommand, expectedAidlFlag) {
		t.Errorf("aidl command %q does not contain %q", aidlCommand, expectedAidlFlag)
	}
}

func TestAidlFlagsArePassedToTheAidlCompiler(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["aidl/foo/IFoo.aidl"],
			aidl: { flags: ["-Werror"], },
		}
	`)

	aidlCommand := ctx.ModuleForTests("foo", "android_common").Rule("aidl").RuleParams.Command
	expectedAidlFlag := "-Werror"
	if !strings.Contains(aidlCommand, expectedAidlFlag) {
		t.Errorf("aidl command %q does not contain %q", aidlCommand, expectedAidlFlag)
	}
}

func TestAidlFlagsWithMinSdkVersion(t *testing.T) {
	fixture := android.GroupFixturePreparers(
		prepareForJavaTest, FixtureWithPrebuiltApis(map[string][]string{"14": {"foo"}}))

	for _, tc := range []struct {
		name       string
		sdkVersion string
		expected   string
	}{
		{"default is current", "", "current"},
		{"use sdk_version", `sdk_version: "14"`, "14"},
		{"system_current", `sdk_version: "system_current"`, "current"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := fixture.RunTestWithBp(t, `
				java_library {
					name: "foo",
					srcs: ["aidl/foo/IFoo.aidl"],
					`+tc.sdkVersion+`
				}
			`)
			aidlCommand := ctx.ModuleForTests("foo", "android_common").Rule("aidl").RuleParams.Command
			expectedAidlFlag := "--min_sdk_version=" + tc.expected
			if !strings.Contains(aidlCommand, expectedAidlFlag) {
				t.Errorf("aidl command %q does not contain %q", aidlCommand, expectedAidlFlag)
			}
		})
	}
}

func TestAidlFlagsMinSdkVersionDroidstubs(t *testing.T) {
	bpTemplate := `
	droidstubs {
		name: "foo-stubs",
		srcs: ["foo.aidl"],
		%s
		system_modules: "none",
	}
	`
	testCases := []struct {
		desc                  string
		sdkVersionBp          string
		minSdkVersionExpected string
	}{
		{
			desc:                  "sdk_version not set, module compiles against private platform APIs",
			sdkVersionBp:          ``,
			minSdkVersionExpected: "10000",
		},
		{
			desc:                  "sdk_version set to none, module does not build against an SDK",
			sdkVersionBp:          `sdk_version: "none",`,
			minSdkVersionExpected: "10000",
		},
	}
	for _, tc := range testCases {
		ctx := prepareForJavaTest.RunTestWithBp(t, fmt.Sprintf(bpTemplate, tc.sdkVersionBp))
		aidlCmd := ctx.ModuleForTests("foo-stubs", "android_common").Rule("aidl").RuleParams.Command
		expected := "--min_sdk_version=" + tc.minSdkVersionExpected
		android.AssertStringDoesContain(t, "aidl command conatins incorrect min_sdk_version for testCse: "+tc.desc, aidlCmd, expected)
	}
}

func TestAidlEnforcePermissions(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["aidl/foo/IFoo.aidl"],
			aidl: { enforce_permissions: true },
		}
	`)

	aidlCommand := ctx.ModuleForTests("foo", "android_common").Rule("aidl").RuleParams.Command
	expectedAidlFlag := "-Wmissing-permission-annotation -Werror"
	if !strings.Contains(aidlCommand, expectedAidlFlag) {
		t.Errorf("aidl command %q does not contain %q", aidlCommand, expectedAidlFlag)
	}
}

func TestAidlEnforcePermissionsException(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["aidl/foo/IFoo.aidl", "aidl/foo/IFoo2.aidl"],
			aidl: { enforce_permissions: true, enforce_permissions_exceptions: ["aidl/foo/IFoo2.aidl"] },
		}
	`)

	aidlCommand := ctx.ModuleForTests("foo", "android_common").Rule("aidl").RuleParams.Command
	expectedAidlFlag := "$$FLAGS -Wmissing-permission-annotation -Werror aidl/foo/IFoo.aidl"
	if !strings.Contains(aidlCommand, expectedAidlFlag) {
		t.Errorf("aidl command %q does not contain %q", aidlCommand, expectedAidlFlag)
	}
	expectedAidlFlag = "$$FLAGS  aidl/foo/IFoo2.aidl"
	if !strings.Contains(aidlCommand, expectedAidlFlag) {
		t.Errorf("aidl command %q does not contain %q", aidlCommand, expectedAidlFlag)
	}
}

func TestDataNativeBinaries(t *testing.T) {
	ctx := android.GroupFixturePreparers(
		prepareForJavaTest,
		android.PrepareForTestWithAllowMissingDependencies).RunTestWithBp(t, `
		java_test_host {
			name: "foo",
			srcs: ["a.java"],
			data_native_bins: ["bin"]
		}

		cc_binary_host {
			name: "bin",
			srcs: ["bin.cpp"],
		}
	`).TestContext

	buildOS := ctx.Config().BuildOS.String()

	test := ctx.ModuleForTests("foo", buildOS+"_common").Module().(*TestHost)
	entries := android.AndroidMkEntriesForTest(t, ctx, test)[0]
	expected := []string{"out/soong/.intermediates/bin/" + buildOS + "_x86_64/bin:bin"}
	actual := entries.EntryMap["LOCAL_COMPATIBILITY_SUPPORT_FILES"]
	android.AssertStringPathsRelativeToTopEquals(t, "LOCAL_COMPATIBILITY_SUPPORT_FILES", ctx.Config(), expected, actual)
}

func TestDefaultInstallable(t *testing.T) {
	ctx, _ := testJava(t, `
		java_test_host {
			name: "foo"
		}
	`)

	buildOS := ctx.Config().BuildOS.String()
	module := ctx.ModuleForTests("foo", buildOS+"_common").Module().(*TestHost)
	assertDeepEquals(t, "Default installable value should be true.", proptools.BoolPtr(true),
		module.properties.Installable)
}

func TestErrorproneEnabled(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			errorprone: {
				enabled: true,
			},
		}
	`)

	javac := ctx.ModuleForTests("foo", "android_common").Description("javac")

	// Test that the errorprone plugins are passed to javac
	expectedSubstring := "-Xplugin:ErrorProne"
	if !strings.Contains(javac.Args["javacFlags"], expectedSubstring) {
		t.Errorf("expected javacFlags to contain %q, got %q", expectedSubstring, javac.Args["javacFlags"])
	}

	// Modules with errorprone { enabled: true } will include errorprone checks
	// in the main javac build rule. Only when RUN_ERROR_PRONE is true will
	// the explicit errorprone build rule be created.
	errorprone := ctx.ModuleForTests("foo", "android_common").MaybeDescription("errorprone")
	if errorprone.RuleParams.Description != "" {
		t.Errorf("expected errorprone build rule to not exist, but it did")
	}
}

func TestErrorproneDisabled(t *testing.T) {
	bp := `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			errorprone: {
				enabled: false,
			},
		}
	`
	ctx := android.GroupFixturePreparers(
		PrepareForTestWithJavaDefaultModules,
		android.FixtureMergeEnv(map[string]string{
			"RUN_ERROR_PRONE": "true",
		}),
	).RunTestWithBp(t, bp)

	javac := ctx.ModuleForTests("foo", "android_common").Description("javac")

	// Test that the errorprone plugins are not passed to javac, like they would
	// be if enabled was true.
	expectedSubstring := "-Xplugin:ErrorProne"
	if strings.Contains(javac.Args["javacFlags"], expectedSubstring) {
		t.Errorf("expected javacFlags to not contain %q, got %q", expectedSubstring, javac.Args["javacFlags"])
	}

	// Check that no errorprone build rule is created, like there would be
	// if enabled was unset and RUN_ERROR_PRONE was true.
	errorprone := ctx.ModuleForTests("foo", "android_common").MaybeDescription("errorprone")
	if errorprone.RuleParams.Description != "" {
		t.Errorf("expected errorprone build rule to not exist, but it did")
	}
}

func TestErrorproneEnabledOnlyByEnvironmentVariable(t *testing.T) {
	bp := `
		java_library {
			name: "foo",
			srcs: ["a.java"],
		}
	`
	ctx := android.GroupFixturePreparers(
		PrepareForTestWithJavaDefaultModules,
		android.FixtureMergeEnv(map[string]string{
			"RUN_ERROR_PRONE": "true",
		}),
	).RunTestWithBp(t, bp)

	javac := ctx.ModuleForTests("foo", "android_common").Description("javac")
	errorprone := ctx.ModuleForTests("foo", "android_common").Description("errorprone")

	// Check that the errorprone plugins are not passed to javac, because they
	// will instead be passed to the separate errorprone compilation
	expectedSubstring := "-Xplugin:ErrorProne"
	if strings.Contains(javac.Args["javacFlags"], expectedSubstring) {
		t.Errorf("expected javacFlags to not contain %q, got %q", expectedSubstring, javac.Args["javacFlags"])
	}

	// Check that the errorprone plugin is enabled
	if !strings.Contains(errorprone.Args["javacFlags"], expectedSubstring) {
		t.Errorf("expected errorprone to contain %q, got %q", expectedSubstring, javac.Args["javacFlags"])
	}
}

func TestDataDeviceBinsBuildsDeviceBinary(t *testing.T) {
	testCases := []struct {
		dataDeviceBinType  string
		depCompileMultilib string
		variants           []string
		expectedError      string
	}{
		{
			dataDeviceBinType:  "first",
			depCompileMultilib: "first",
			variants:           []string{"android_arm64_armv8-a"},
		},
		{
			dataDeviceBinType:  "first",
			depCompileMultilib: "both",
			variants:           []string{"android_arm64_armv8-a"},
		},
		{
			// this is true because our testing framework is set up with
			// Targets ~ [<64bit target>, <32bit target>], where 64bit is "first"
			dataDeviceBinType:  "first",
			depCompileMultilib: "32",
			expectedError:      `Android.bp:2:3: dependency "bar" of "foo" missing variant`,
		},
		{
			dataDeviceBinType:  "first",
			depCompileMultilib: "64",
			variants:           []string{"android_arm64_armv8-a"},
		},
		{
			dataDeviceBinType:  "both",
			depCompileMultilib: "both",
			variants: []string{
				"android_arm_armv7-a-neon",
				"android_arm64_armv8-a",
			},
		},
		{
			dataDeviceBinType:  "both",
			depCompileMultilib: "32",
			expectedError:      `Android.bp:2:3: dependency "bar" of "foo" missing variant`,
		},
		{
			dataDeviceBinType:  "both",
			depCompileMultilib: "64",
			expectedError:      `Android.bp:2:3: dependency "bar" of "foo" missing variant`,
		},
		{
			dataDeviceBinType:  "both",
			depCompileMultilib: "first",
			expectedError:      `Android.bp:2:3: dependency "bar" of "foo" missing variant`,
		},
		{
			dataDeviceBinType:  "32",
			depCompileMultilib: "32",
			variants:           []string{"android_arm_armv7-a-neon"},
		},
		{
			dataDeviceBinType:  "32",
			depCompileMultilib: "first",
			expectedError:      `Android.bp:2:3: dependency "bar" of "foo" missing variant`,
		},
		{
			dataDeviceBinType:  "32",
			depCompileMultilib: "both",
			variants:           []string{"android_arm_armv7-a-neon"},
		},
		{
			dataDeviceBinType:  "32",
			depCompileMultilib: "64",
			expectedError:      `Android.bp:2:3: dependency "bar" of "foo" missing variant`,
		},
		{
			dataDeviceBinType:  "64",
			depCompileMultilib: "64",
			variants:           []string{"android_arm64_armv8-a"},
		},
		{
			dataDeviceBinType:  "64",
			depCompileMultilib: "both",
			variants:           []string{"android_arm64_armv8-a"},
		},
		{
			dataDeviceBinType:  "64",
			depCompileMultilib: "first",
			variants:           []string{"android_arm64_armv8-a"},
		},
		{
			dataDeviceBinType:  "64",
			depCompileMultilib: "32",
			expectedError:      `Android.bp:2:3: dependency "bar" of "foo" missing variant`,
		},
		{
			dataDeviceBinType:  "prefer32",
			depCompileMultilib: "32",
			variants:           []string{"android_arm_armv7-a-neon"},
		},
		{
			dataDeviceBinType:  "prefer32",
			depCompileMultilib: "both",
			variants:           []string{"android_arm_armv7-a-neon"},
		},
		{
			dataDeviceBinType:  "prefer32",
			depCompileMultilib: "first",
			expectedError:      `Android.bp:2:3: dependency "bar" of "foo" missing variant`,
		},
		{
			dataDeviceBinType:  "prefer32",
			depCompileMultilib: "64",
			expectedError:      `Android.bp:2:3: dependency "bar" of "foo" missing variant`,
		},
	}

	bpTemplate := `
		java_test_host {
			name: "foo",
			srcs: ["test.java"],
			data_device_bins_%s: ["bar"],
		}

		cc_binary {
			name: "bar",
			compile_multilib: "%s",
		}
	`

	for _, tc := range testCases {
		bp := fmt.Sprintf(bpTemplate, tc.dataDeviceBinType, tc.depCompileMultilib)

		errorHandler := android.FixtureExpectsNoErrors
		if tc.expectedError != "" {
			errorHandler = android.FixtureExpectsAtLeastOneErrorMatchingPattern(tc.expectedError)
		}

		testName := fmt.Sprintf(`data_device_bins_%s with compile_multilib:"%s"`, tc.dataDeviceBinType, tc.depCompileMultilib)
		t.Run(testName, func(t *testing.T) {
			ctx := android.GroupFixturePreparers(PrepareForIntegrationTestWithJava).
				ExtendWithErrorHandler(errorHandler).
				RunTestWithBp(t, bp)
			if tc.expectedError != "" {
				return
			}

			buildOS := ctx.Config.BuildOS.String()
			fooVariant := ctx.ModuleForTests("foo", buildOS+"_common")
			fooMod := fooVariant.Module().(*TestHost)
			entries := android.AndroidMkEntriesForTest(t, ctx.TestContext, fooMod)[0]

			expectedAutogenConfig := `<option name="push-file" key="bar" value="/data/local/tests/unrestricted/foo/bar" />`
			autogen := fooVariant.Rule("autogen")
			if !strings.Contains(autogen.Args["extraConfigs"], expectedAutogenConfig) {
				t.Errorf("foo extraConfigs %v does not contain %q", autogen.Args["extraConfigs"], expectedAutogenConfig)
			}

			expectedData := []string{}
			for _, variant := range tc.variants {
				barVariant := ctx.ModuleForTests("bar", variant)
				relocated := barVariant.Output("bar")
				expectedInput := fmt.Sprintf("out/soong/.intermediates/bar/%s/unstripped/bar", variant)
				android.AssertPathRelativeToTopEquals(t, "relocation input", expectedInput, relocated.Input)

				expectedData = append(expectedData, fmt.Sprintf("out/soong/.intermediates/bar/%s/bar:bar", variant))
			}

			actualData := entries.EntryMap["LOCAL_COMPATIBILITY_SUPPORT_FILES"]
			android.AssertStringPathsRelativeToTopEquals(t, "LOCAL_TEST_DATA", ctx.Config, expectedData, actualData)
		})
	}
}

func TestDeviceBinaryWrapperGeneration(t *testing.T) {
	// Scenario 1: java_binary has main_class property in its bp
	ctx, _ := testJava(t, `
		java_binary {
			name: "foo",
			srcs: ["foo.java"],
			main_class: "foo.bar.jb",
		}
	`)
	wrapperPath := fmt.Sprint(ctx.ModuleForTests("foo", "android_common").AllOutputs())
	if !strings.Contains(wrapperPath, "foo.sh") {
		t.Errorf("wrapper file foo.sh is not generated")
	}

	// Scenario 2: java_binary has neither wrapper nor main_class, its build
	// is expected to be failed.
	testJavaError(t, "main_class property is required for device binary if no default wrapper is assigned", `
		java_binary {
			name: "foo",
			srcs: ["foo.java"],
		}`)
}

func TestJavaApiContributionEmptyApiFile(t *testing.T) {
	android.GroupFixturePreparers(
		prepareForJavaTest,
		android.FixtureMergeEnv(
			map[string]string{
				"DISABLE_STUB_VALIDATION": "true",
			},
		),
	).ExtendWithErrorHandler(android.FixtureExpectsAtLeastOneErrorMatchingPattern(
		"Error: foo has an empty api file.",
	)).RunTestWithBp(t, `
		java_api_contribution {
			name: "foo",
		}
		java_api_library {
			name: "bar",
			api_surface: "public",
			api_contributions: ["foo"],
			stubs_type: "everything",
		}
	`)
}

func TestJavaApiLibraryAndProviderLink(t *testing.T) {
	provider_bp_a := `
	java_api_contribution {
		name: "foo1",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	provider_bp_b := `java_api_contribution {
		name: "foo2",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	ctx := android.GroupFixturePreparers(
		prepareForJavaTest,
		android.FixtureMergeMockFs(
			map[string][]byte{
				"a/Android.bp": []byte(provider_bp_a),
				"b/Android.bp": []byte(provider_bp_b),
			},
		),
		android.FixtureMergeEnv(
			map[string]string{
				"DISABLE_STUB_VALIDATION": "true",
			},
		),
	).RunTestWithBp(t, `
		java_api_library {
			name: "bar1",
			api_surface: "public",
			api_contributions: ["foo1"],
			stubs_type: "everything",
		}

		java_api_library {
			name: "bar2",
			api_surface: "system",
			api_contributions: ["foo1", "foo2"],
			stubs_type: "everything",
		}
	`)

	testcases := []struct {
		moduleName         string
		sourceTextFileDirs []string
	}{
		{
			moduleName:         "bar1",
			sourceTextFileDirs: []string{"a/current.txt"},
		},
		{
			moduleName:         "bar2",
			sourceTextFileDirs: []string{"a/current.txt", "b/current.txt"},
		},
	}
	for _, c := range testcases {
		m := ctx.ModuleForTests(c.moduleName, "android_common")
		manifest := m.Output("metalava.sbox.textproto")
		sboxProto := android.RuleBuilderSboxProtoForTests(t, ctx.TestContext, manifest)
		manifestCommand := sboxProto.Commands[0].GetCommand()
		sourceFilesFlag := "--source-files " + strings.Join(c.sourceTextFileDirs, " ")
		android.AssertStringDoesContain(t, "source text files not present", manifestCommand, sourceFilesFlag)
	}
}

func TestJavaApiLibraryAndDefaultsLink(t *testing.T) {
	provider_bp_a := `
	java_api_contribution {
		name: "foo1",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	provider_bp_b := `
	java_api_contribution {
		name: "foo2",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	provider_bp_c := `
	java_api_contribution {
		name: "foo3",
		api_file: "system-current.txt",
		api_surface: "system",
	}
	`
	provider_bp_d := `
	java_api_contribution {
		name: "foo4",
		api_file: "system-current.txt",
		api_surface: "system",
	}
	`
	ctx := android.GroupFixturePreparers(
		prepareForJavaTest,
		android.FixtureMergeMockFs(
			map[string][]byte{
				"a/Android.bp": []byte(provider_bp_a),
				"b/Android.bp": []byte(provider_bp_b),
				"c/Android.bp": []byte(provider_bp_c),
				"d/Android.bp": []byte(provider_bp_d),
			},
		),
		android.FixtureMergeEnv(
			map[string]string{
				"DISABLE_STUB_VALIDATION": "true",
			},
		),
	).RunTestWithBp(t, `
		java_defaults {
			name: "baz1",
			api_surface: "public",
			api_contributions: ["foo1", "foo2"],
		}

		java_defaults {
			name: "baz2",
			api_surface: "system",
			api_contributions: ["foo3"],
		}

		java_api_library {
			name: "bar1",
			api_surface: "public",
			api_contributions: ["foo1"],
			stubs_type: "everything",
		}

		java_api_library {
			name: "bar2",
			api_surface: "public",
			defaults:["baz1"],
			stubs_type: "everything",
		}

		java_api_library {
			name: "bar3",
			api_surface: "system",
			defaults:["baz1", "baz2"],
			api_contributions: ["foo4"],
			stubs_type: "everything",
		}
	`)

	testcases := []struct {
		moduleName         string
		sourceTextFileDirs []string
	}{
		{
			moduleName:         "bar1",
			sourceTextFileDirs: []string{"a/current.txt"},
		},
		{
			moduleName:         "bar2",
			sourceTextFileDirs: []string{"a/current.txt", "b/current.txt"},
		},
		{
			moduleName: "bar3",
			// API text files need to be sorted from the narrower api scope to the wider api scope
			sourceTextFileDirs: []string{"a/current.txt", "b/current.txt", "c/system-current.txt", "d/system-current.txt"},
		},
	}
	for _, c := range testcases {
		m := ctx.ModuleForTests(c.moduleName, "android_common")
		manifest := m.Output("metalava.sbox.textproto")
		sboxProto := android.RuleBuilderSboxProtoForTests(t, ctx.TestContext, manifest)
		manifestCommand := sboxProto.Commands[0].GetCommand()
		sourceFilesFlag := "--source-files " + strings.Join(c.sourceTextFileDirs, " ")
		android.AssertStringDoesContain(t, "source text files not present", manifestCommand, sourceFilesFlag)
	}
}

func TestJavaApiLibraryJarGeneration(t *testing.T) {
	provider_bp_a := `
	java_api_contribution {
		name: "foo1",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	provider_bp_b := `
	java_api_contribution {
		name: "foo2",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	ctx := android.GroupFixturePreparers(
		prepareForJavaTest,
		android.FixtureMergeMockFs(
			map[string][]byte{
				"a/Android.bp": []byte(provider_bp_a),
				"b/Android.bp": []byte(provider_bp_b),
			},
		),
		android.FixtureMergeEnv(
			map[string]string{
				"DISABLE_STUB_VALIDATION": "true",
			},
		),
	).RunTestWithBp(t, `
		java_api_library {
			name: "bar1",
			api_surface: "public",
			api_contributions: ["foo1"],
			stubs_type: "everything",
		}

		java_api_library {
			name: "bar2",
			api_surface: "system",
			api_contributions: ["foo1", "foo2"],
			stubs_type: "everything",
		}
	`)

	testcases := []struct {
		moduleName    string
		outputJarName string
	}{
		{
			moduleName:    "bar1",
			outputJarName: "bar1/bar1.jar",
		},
		{
			moduleName:    "bar2",
			outputJarName: "bar2/bar2.jar",
		},
	}
	for _, c := range testcases {
		m := ctx.ModuleForTests(c.moduleName, "android_common")
		outputs := fmt.Sprint(m.AllOutputs())
		if !strings.Contains(outputs, c.outputJarName) {
			t.Errorf("Module output does not contain expected jar %s", c.outputJarName)
		}
	}
}

func TestJavaApiLibraryLibsLink(t *testing.T) {
	provider_bp_a := `
	java_api_contribution {
		name: "foo1",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	provider_bp_b := `
	java_api_contribution {
		name: "foo2",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	lib_bp_a := `
	java_library {
		name: "lib1",
		srcs: ["Lib.java"],
	}
	`
	lib_bp_b := `
	java_library {
		name: "lib2",
		srcs: ["Lib.java"],
	}
	`

	ctx := android.GroupFixturePreparers(
		prepareForJavaTest,
		android.FixtureMergeMockFs(
			map[string][]byte{
				"a/Android.bp": []byte(provider_bp_a),
				"b/Android.bp": []byte(provider_bp_b),
				"c/Android.bp": []byte(lib_bp_a),
				"c/Lib.java":   {},
				"d/Android.bp": []byte(lib_bp_b),
				"d/Lib.java":   {},
			},
		),
		android.FixtureMergeEnv(
			map[string]string{
				"DISABLE_STUB_VALIDATION": "true",
			},
		),
	).RunTestWithBp(t, `
		java_api_library {
			name: "bar1",
			api_surface: "public",
			api_contributions: ["foo1"],
			libs: ["lib1"],
			stubs_type: "everything",
		}

		java_api_library {
			name: "bar2",
			api_surface: "system",
			api_contributions: ["foo1", "foo2"],
			libs: ["lib1", "lib2", "bar1"],
			stubs_type: "everything",
		}
	`)

	testcases := []struct {
		moduleName        string
		classPathJarNames []string
	}{
		{
			moduleName:        "bar1",
			classPathJarNames: []string{"lib1.jar"},
		},
		{
			moduleName:        "bar2",
			classPathJarNames: []string{"lib1.jar", "lib2.jar", "bar1/bar1.jar"},
		},
	}
	for _, c := range testcases {
		m := ctx.ModuleForTests(c.moduleName, "android_common")
		javacRules := m.Rule("javac")
		classPathArgs := javacRules.Args["classpath"]
		for _, jarName := range c.classPathJarNames {
			if !strings.Contains(classPathArgs, jarName) {
				t.Errorf("Module output does not contain expected jar %s", jarName)
			}
		}
	}
}

func TestJavaApiLibraryStaticLibsLink(t *testing.T) {
	provider_bp_a := `
	java_api_contribution {
		name: "foo1",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	provider_bp_b := `
	java_api_contribution {
		name: "foo2",
		api_file: "current.txt",
		api_surface: "public",
	}
	`
	lib_bp_a := `
	java_library {
		name: "lib1",
		srcs: ["Lib.java"],
	}
	`
	lib_bp_b := `
	java_library {
		name: "lib2",
		srcs: ["Lib.java"],
	}
	`

	ctx := android.GroupFixturePreparers(
		prepareForJavaTest,
		android.FixtureMergeMockFs(
			map[string][]byte{
				"a/Android.bp": []byte(provider_bp_a),
				"b/Android.bp": []byte(provider_bp_b),
				"c/Android.bp": []byte(lib_bp_a),
				"c/Lib.java":   {},
				"d/Android.bp": []byte(lib_bp_b),
				"d/Lib.java":   {},
			},
		),
		android.FixtureMergeEnv(
			map[string]string{
				"DISABLE_STUB_VALIDATION": "true",
			},
		),
	).RunTestWithBp(t, `
		java_api_library {
			name: "bar1",
			api_surface: "public",
			api_contributions: ["foo1"],
			static_libs: ["lib1"],
			stubs_type: "everything",
		}

		java_api_library {
			name: "bar2",
			api_surface: "system",
			api_contributions: ["foo1", "foo2"],
			static_libs: ["lib1", "lib2", "bar1"],
			stubs_type: "everything",
		}
	`)

	testcases := []struct {
		moduleName        string
		staticLibJarNames []string
	}{
		{
			moduleName:        "bar1",
			staticLibJarNames: []string{"lib1.jar"},
		},
		{
			moduleName:        "bar2",
			staticLibJarNames: []string{"lib1.jar", "lib2.jar", "bar1/bar1.jar"},
		},
	}
	for _, c := range testcases {
		m := ctx.ModuleForTests(c.moduleName, "android_common")
		mergeZipsCommand := m.Rule("merge_zips").RuleParams.Command
		for _, jarName := range c.staticLibJarNames {
			if !strings.Contains(mergeZipsCommand, jarName) {
				t.Errorf("merge_zips command does not contain expected jar %s", jarName)
			}
		}
	}
}

func TestTransitiveSrcFiles(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "a",
			srcs: ["a.java"],
		}
		java_library {
			name: "b",
			srcs: ["b.java"],
		}
		java_library {
			name: "c",
			srcs: ["c.java"],
			libs: ["a"],
			static_libs: ["b"],
		}
	`)
	c := ctx.ModuleForTests("c", "android_common").Module()
	javaInfo, _ := android.OtherModuleProvider(ctx, c, JavaInfoProvider)
	transitiveSrcFiles := android.Paths(javaInfo.TransitiveSrcFiles.ToList())
	android.AssertArrayString(t, "unexpected jar deps", []string{"b.java", "c.java"}, transitiveSrcFiles.Strings())
}

func TestTradefedOptions(t *testing.T) {
	result := PrepareForTestWithJavaBuildComponents.RunTestWithBp(t, `
java_test_host {
	name: "foo",
	test_options: {
		tradefed_options: [
			{
				name: "exclude-path",
				value: "org/apache"
			}
		]
	}
}
`)

	buildOS := result.Config.BuildOS.String()
	args := result.ModuleForTests("foo", buildOS+"_common").
		Output("out/soong/.intermediates/foo/" + buildOS + "_common/foo.config").Args
	expected := proptools.NinjaAndShellEscape("<option name=\"exclude-path\" value=\"org/apache\" />")
	if args["extraConfigs"] != expected {
		t.Errorf("Expected args[\"extraConfigs\"] to equal %q, was %q", expected, args["extraConfigs"])
	}
}

func TestTestRunnerOptions(t *testing.T) {
	result := PrepareForTestWithJavaBuildComponents.RunTestWithBp(t, `
java_test_host {
	name: "foo",
	test_options: {
		test_runner_options: [
			{
				name: "test-timeout",
				value: "10m"
			}
		]
	}
}
`)

	buildOS := result.Config.BuildOS.String()
	args := result.ModuleForTests("foo", buildOS+"_common").
		Output("out/soong/.intermediates/foo/" + buildOS + "_common/foo.config").Args
	expected := proptools.NinjaAndShellEscape("<option name=\"test-timeout\" value=\"10m\" />\\n        ")
	if args["extraTestRunnerConfigs"] != expected {
		t.Errorf("Expected args[\"extraTestRunnerConfigs\"] to equal %q, was %q", expected, args["extraTestRunnerConfigs"])
	}
}

func TestJavaLibraryWithResourcesStem(t *testing.T) {
	ctx, _ := testJavaWithFS(t, `
    java_library {
        name: "foo",
        java_resource_dirs: ["test-jar"],
        stem: "test",
    }
    `,
		map[string][]byte{
			"test-jar/test/resource.txt": nil,
		})

	m := ctx.ModuleForTests("foo", "android_common")
	outputs := fmt.Sprint(m.AllOutputs())
	if !strings.Contains(outputs, "test.jar") {
		t.Errorf("Module output does not contain expected jar %s", "test.jar")
	}
}

func TestHeadersOnly(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			headers_only: true,
		}
	`)

	turbine := ctx.ModuleForTests("foo", "android_common").Rule("turbine")
	if len(turbine.Inputs) != 1 || turbine.Inputs[0].String() != "a.java" {
		t.Errorf(`foo inputs %v != ["a.java"]`, turbine.Inputs)
	}

	javac := ctx.ModuleForTests("foo", "android_common").MaybeRule("javac")
	android.AssertDeepEquals(t, "javac rule", nil, javac.Rule)
}

func TestJavaApiContributionImport(t *testing.T) {
	ctx := android.GroupFixturePreparers(
		prepareForJavaTest,
		android.FixtureMergeEnv(
			map[string]string{
				"DISABLE_STUB_VALIDATION": "true",
			},
		),
	).RunTestWithBp(t, `
		java_api_library {
			name: "foo",
			api_contributions: ["bar"],
			stubs_type: "everything",
		}
		java_api_contribution_import {
			name: "bar",
			api_file: "current.txt",
			api_surface: "public",
		}
	`)
	m := ctx.ModuleForTests("foo", "android_common")
	manifest := m.Output("metalava.sbox.textproto")
	sboxProto := android.RuleBuilderSboxProtoForTests(t, ctx.TestContext, manifest)
	manifestCommand := sboxProto.Commands[0].GetCommand()
	sourceFilesFlag := "--source-files current.txt"
	android.AssertStringDoesContain(t, "source text files not present", manifestCommand, sourceFilesFlag)
}

func TestJavaApiLibraryApiFilesSorting(t *testing.T) {
	ctx, _ := testJava(t, `
		java_api_library {
			name: "foo",
			api_contributions: [
				"system-server-api-stubs-docs-non-updatable.api.contribution",
				"test-api-stubs-docs-non-updatable.api.contribution",
				"system-api-stubs-docs-non-updatable.api.contribution",
				"module-lib-api-stubs-docs-non-updatable.api.contribution",
				"api-stubs-docs-non-updatable.api.contribution",
			],
			stubs_type: "everything",
		}
	`)
	m := ctx.ModuleForTests("foo", "android_common")
	manifest := m.Output("metalava.sbox.textproto")
	sboxProto := android.RuleBuilderSboxProtoForTests(t, ctx, manifest)
	manifestCommand := sboxProto.Commands[0].GetCommand()

	// Api files are sorted from the narrowest api scope to the widest api scope.
	// test api and module lib api surface do not have subset/superset relationship,
	// but they will never be passed as inputs at the same time.
	sourceFilesFlag := "--source-files default/java/api/current.txt " +
		"default/java/api/system-current.txt default/java/api/test-current.txt " +
		"default/java/api/module-lib-current.txt default/java/api/system-server-current.txt"
	android.AssertStringDoesContain(t, "source text files not in api scope order", manifestCommand, sourceFilesFlag)
}

func TestSdkLibraryProvidesSystemModulesToApiLibrary(t *testing.T) {
	result := android.GroupFixturePreparers(
		prepareForJavaTest,
		PrepareForTestWithJavaSdkLibraryFiles,
		FixtureWithLastReleaseApis("foo"),
		android.FixtureMergeMockFs(
			map[string][]byte{
				"A.java": nil,
			},
		),
	).RunTestWithBp(t, `
		java_library {
			name: "bar",
			srcs: ["a.java"],
		}
		java_system_modules {
			name: "baz",
			libs: ["bar"],
		}
		java_sdk_library {
			name: "foo",
			srcs: ["A.java"],
			system_modules: "baz",
		}
	`)

	checkBootClasspathForLibWithSystemModule(t, result.TestContext, apiScopePublic.apiLibraryModuleName("foo"), "/bar.jar")
}

func TestApiLibraryDroidstubsDependency(t *testing.T) {
	result := android.GroupFixturePreparers(
		prepareForJavaTest,
		PrepareForTestWithJavaSdkLibraryFiles,
		FixtureWithLastReleaseApis("foo"),
		android.FixtureMergeMockFs(
			map[string][]byte{
				"A.java": nil,
			},
		),
	).RunTestWithBp(t, `
		java_api_library {
			name: "foo",
			api_contributions: [
				"api-stubs-docs-non-updatable.api.contribution",
			],
			enable_validation: true,
			stubs_type: "everything",
		}
		java_api_library {
			name: "bar",
			api_contributions: [
				"api-stubs-docs-non-updatable.api.contribution",
			],
			enable_validation: false,
			stubs_type: "everything",
		}
	`)

	currentApiTimestampPath := "api-stubs-docs-non-updatable/android_common/everything/check_current_api.timestamp"
	foo := result.ModuleForTests("foo", "android_common").Module().(*ApiLibrary)
	fooValidationPathsString := strings.Join(foo.validationPaths.Strings(), " ")
	bar := result.ModuleForTests("bar", "android_common").Module().(*ApiLibrary)
	barValidationPathsString := strings.Join(bar.validationPaths.Strings(), " ")
	android.AssertStringDoesContain(t,
		"Module expected to have validation",
		fooValidationPathsString,
		currentApiTimestampPath,
	)
	android.AssertStringDoesNotContain(t,
		"Module expected to not have validation",
		barValidationPathsString,
		currentApiTimestampPath,
	)
}

func TestDisableFromTextStubForCoverageBuild(t *testing.T) {
	result := android.GroupFixturePreparers(
		prepareForJavaTest,
		PrepareForTestWithJavaSdkLibraryFiles,
		PrepareForTestWithJacocoInstrumentation,
		FixtureWithLastReleaseApis("foo"),
		android.FixtureModifyConfig(func(config android.Config) {
			config.SetBuildFromTextStub(true)
		}),
		android.FixtureModifyEnv(func(env map[string]string) {
			env["EMMA_INSTRUMENT"] = "true"
		}),
	).RunTestWithBp(t, `
		java_sdk_library {
			name: "foo",
			srcs: ["A.java"],
		}
	`)
	android.AssertBoolEquals(t, "stub module expected to depend on from-source stub",
		true, CheckModuleHasDependency(t, result.TestContext,
			apiScopePublic.stubsLibraryModuleName("foo"), "android_common",
			apiScopePublic.sourceStubsLibraryModuleName("foo")))

	android.AssertBoolEquals(t, "stub module expected to not depend on from-text stub",
		false, CheckModuleHasDependency(t, result.TestContext,
			apiScopePublic.stubsLibraryModuleName("foo"), "android_common",
			apiScopePublic.apiLibraryModuleName("foo")))
}

func TestMultiplePrebuilts(t *testing.T) {
	bp := `
		// an rdep
		java_library {
			name: "foo",
			libs: ["bar"],
		}

		// multiple variations of dep
		// source
		java_library {
			name: "bar",
			srcs: ["bar.java"],
		}
		// prebuilt "v1"
		java_import {
			name: "bar",
			jars: ["bar.jar"],
		}
		// prebuilt "v2"
		java_import {
			name: "bar.v2",
			source_module_name: "bar",
			jars: ["bar.v1.jar"],
		}

		// selectors
		apex_contributions {
			name: "myapex_contributions",
			contents: ["%v"],
		}
	`
	hasDep := func(ctx *android.TestResult, m android.Module, wantDep android.Module) bool {
		t.Helper()
		var found bool
		ctx.VisitDirectDeps(m, func(dep blueprint.Module) {
			if dep == wantDep {
				found = true
			}
		})
		return found
	}

	hasFileWithStem := func(m android.TestingModule, stem string) bool {
		t.Helper()
		for _, o := range m.AllOutputs() {
			_, file := filepath.Split(o)
			if file == stem+".jar" {
				return true
			}
		}
		return false
	}

	testCases := []struct {
		desc                   string
		selectedDependencyName string
		expectedDependencyName string
	}{
		{
			desc:                   "Source library is selected using apex_contributions",
			selectedDependencyName: "bar",
			expectedDependencyName: "bar",
		},
		{
			desc:                   "Prebuilt library v1 is selected using apex_contributions",
			selectedDependencyName: "prebuilt_bar",
			expectedDependencyName: "prebuilt_bar",
		},
		{
			desc:                   "Prebuilt library v2 is selected using apex_contributions",
			selectedDependencyName: "prebuilt_bar.v2",
			expectedDependencyName: "prebuilt_bar.v2",
		},
	}

	for _, tc := range testCases {
		ctx := android.GroupFixturePreparers(
			prepareForJavaTest,
			android.PrepareForTestWithBuildFlag("RELEASE_APEX_CONTRIBUTIONS_ADSERVICES", "myapex_contributions"),
		).RunTestWithBp(t, fmt.Sprintf(bp, tc.selectedDependencyName))

		// check that rdep gets the correct variation of dep
		foo := ctx.ModuleForTests("foo", "android_common")
		expectedDependency := ctx.ModuleForTests(tc.expectedDependencyName, "android_common")
		android.AssertBoolEquals(t, fmt.Sprintf("expected dependency from %s to %s\n", foo.Module().Name(), tc.expectedDependencyName), true, hasDep(ctx, foo.Module(), expectedDependency.Module()))

		// check that output file of dep is always bar.jar
		// The filename should be agnostic to source/prebuilt/prebuilt_version
		android.AssertBoolEquals(t, fmt.Sprintf("could not find bar.jar in outputs of %s. All Outputs %v\n", tc.expectedDependencyName, expectedDependency.AllOutputs()), true, hasFileWithStem(expectedDependency, "bar"))

		// check LOCAL_MODULE of the selected module name
		// the prebuilt should have the same LOCAL_MODULE when exported to make
		entries := android.AndroidMkEntriesForTest(t, ctx.TestContext, expectedDependency.Module())[0]
		android.AssertStringEquals(t, "unexpected LOCAL_MODULE", "bar", entries.EntryMap["LOCAL_MODULE"][0])
	}
}

func TestMultiplePlatformCompatConfigPrebuilts(t *testing.T) {
	bp := `
		// multiple variations of platform_compat_config
		// source
		platform_compat_config {
			name: "myconfig",
		}
		// prebuilt "v1"
		prebuilt_platform_compat_config {
			name: "myconfig",
			metadata: "myconfig.xml",
		}
		// prebuilt "v2"
		prebuilt_platform_compat_config {
			name: "myconfig.v2",
			source_module_name: "myconfig", // without source_module_name, the singleton will merge two .xml files
			metadata: "myconfig.v2.xml",
		}

		// selectors
		apex_contributions {
			name: "myapex_contributions",
			contents: ["%v"],
		}
	`
	testCases := []struct {
		desc                            string
		selectedDependencyName          string
		expectedPlatformCompatConfigXml string
	}{
		{
			desc:                            "Source platform_compat_config is selected using apex_contributions",
			selectedDependencyName:          "myconfig",
			expectedPlatformCompatConfigXml: "out/soong/.intermediates/myconfig/android_common/myconfig_meta.xml",
		},
		{
			desc:                            "Prebuilt platform_compat_config v1 is selected using apex_contributions",
			selectedDependencyName:          "prebuilt_myconfig",
			expectedPlatformCompatConfigXml: "myconfig.xml",
		},
		{
			desc:                            "Prebuilt platform_compat_config v2 is selected using apex_contributions",
			selectedDependencyName:          "prebuilt_myconfig.v2",
			expectedPlatformCompatConfigXml: "myconfig.v2.xml",
		},
	}

	for _, tc := range testCases {
		ctx := android.GroupFixturePreparers(
			prepareForJavaTest,
			PrepareForTestWithPlatformCompatConfig,
			android.PrepareForTestWithBuildFlag("RELEASE_APEX_CONTRIBUTIONS_ADSERVICES", "myapex_contributions"),
		).RunTestWithBp(t, fmt.Sprintf(bp, tc.selectedDependencyName))

		mergedGlobalConfig := ctx.SingletonForTests("platform_compat_config_singleton").Output("compat_config/merged_compat_config.xml")
		android.AssertIntEquals(t, "The merged compat config file should only have a single dependency", 1, len(mergedGlobalConfig.Implicits))
		android.AssertStringEquals(t, "The merged compat config file is missing the appropriate platform compat config", mergedGlobalConfig.Implicits[0].String(), tc.expectedPlatformCompatConfigXml)
	}
}

func TestApiLibraryAconfigDeclarations(t *testing.T) {
	result := android.GroupFixturePreparers(
		prepareForJavaTest,
		android.FixtureModifyProductVariables(func(variables android.FixtureProductVariables) {
		}),
		android.FixtureMergeMockFs(map[string][]byte{
			"a/A.java":      nil,
			"a/current.txt": nil,
			"a/removed.txt": nil,
		}),
	).RunTestWithBp(t, `
	aconfig_declarations {
		name: "bar",
		package: "com.example.package",
		container: "com.android.foo",
		srcs: [
			"bar.aconfig",
		],
	}
	java_api_contribution {
		name: "baz",
		api_file: "a/current.txt",
		api_surface: "public",
	}
	java_api_library {
		name: "foo",
		api_surface: "public",
		api_contributions: [
			"baz",
		],
		aconfig_declarations: [
			"bar",
		],
		stubs_type: "exportable",
		enable_validation: false,
	}
	`)

	// Check if java_api_library depends on aconfig_declarations
	android.AssertBoolEquals(t, "foo expected to depend on bar",
		CheckModuleHasDependency(t, result.TestContext, "foo", "android_common", "bar"), true)

	m := result.ModuleForTests("foo", "android_common")
	android.AssertStringDoesContain(t, "foo generates revert annotations file",
		strings.Join(m.AllOutputs(), ""), "revert-annotations-exportable.txt")

	// revert-annotations.txt passed to exportable stubs generation metalava command
	manifest := m.Output("metalava.sbox.textproto")
	cmdline := String(android.RuleBuilderSboxProtoForTests(t, result.TestContext, manifest).Commands[0].Command)
	android.AssertStringDoesContain(t, "flagged api hide command not included", cmdline, "revert-annotations-exportable.txt")
}

func TestTestOnly(t *testing.T) {
	t.Parallel()
	ctx := android.GroupFixturePreparers(
		prepareForJavaTest,
	).RunTestWithBp(t, `
                // These should be test-only
		java_library {
			name: "lib1-test-only",
                        srcs: ["a.java"],
                        test_only: true,
		}
                java_test {
                        name: "java-test",
                }
                java_test_host {
                        name: "java-test-host",
                }
                java_test_helper_library {
                        name: "helper-library",
                }
                java_binary {
                        name: "java-data-binary",
			srcs: ["foo.java"],
			main_class: "foo.bar.jb",
                        test_only: true,
                }

                // These are NOT
		java_library {
			name: "lib2-app",
                        srcs: ["b.java"],
		}
		java_import {
			name: "bar",
			jars: ["bar.jar"],
		}
                java_binary {
                        name: "java-binary",
			srcs: ["foo.java"],
			main_class: "foo.bar.jb",
                }
	`)

	expectedTestOnlyModules := []string{
		"lib1-test-only",
		"java-test",
		"java-test-host",
		"helper-library",
		"java-data-binary",
	}
	expectedTopLevelTests := []string{
		"java-test",
		"java-test-host",
	}
	assertTestOnlyAndTopLevel(t, ctx, expectedTestOnlyModules, expectedTopLevelTests)
}

// Don't allow setting test-only on things that are always tests or never tests.
func TestInvalidTestOnlyTargets(t *testing.T) {
	testCases := []string{
		` java_test {  name: "java-test", test_only: true, srcs: ["foo.java"],  } `,
		` java_test_host {  name: "java-test-host", test_only: true, srcs: ["foo.java"],  } `,
		` java_test_import {  name: "java-test-import", test_only: true, } `,
		` java_api_library {  name: "java-api-library", test_only: true, } `,
		` java_test_helper_library { name: "test-help-lib", test_only: true, } `,
		` java_defaults { name: "java-defaults", test_only: true, } `,
	}

	for i, bp := range testCases {
		android.GroupFixturePreparers(prepareForJavaTest).
			ExtendWithErrorHandler(
				expectOneError("unrecognized property \"test_only\"",
					fmt.Sprintf("testcase: %d", i))).
			RunTestWithBp(t, bp)
	}
}

// Expect exactly one that matches 'expected'.
// Append 'msg' to the Errorf that printed.
func expectOneError(expected string, msg string) android.FixtureErrorHandler {
	return android.FixtureCustomErrorHandler(func(t *testing.T, result *android.TestResult) {
		t.Helper()
		if len(result.Errs) != 1 {
			t.Errorf("Expected exactly one error, but found: %d when  setting test_only on: %s", len(result.Errs), msg)
			return
		}
		actualErrMsg := result.Errs[0].Error()
		if !strings.Contains(actualErrMsg, expected) {
			t.Errorf("Different error than expected.  Received: [%v] on %s expected: %s", actualErrMsg, msg, expected)
		}
	})
}

func TestJavaLibHostWithStem(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library_host {
			name: "foo",
			srcs: ["a.java"],
			stem: "foo-new",
		}
	`)

	buildOS := ctx.Config().BuildOS.String()
	foo := ctx.ModuleForTests("foo", buildOS+"_common")

	outputs := fmt.Sprint(foo.AllOutputs())
	if !strings.Contains(outputs, "foo-new.jar") {
		t.Errorf("Module output does not contain expected jar %s", "foo-new.jar")
	}
}

func TestJavaLibWithStem(t *testing.T) {
	ctx, _ := testJava(t, `
		java_library {
			name: "foo",
			srcs: ["a.java"],
			stem: "foo-new",
		}
	`)

	foo := ctx.ModuleForTests("foo", "android_common")

	outputs := fmt.Sprint(foo.AllOutputs())
	if !strings.Contains(outputs, "foo-new.jar") {
		t.Errorf("Module output does not contain expected jar %s", "foo-new.jar")
	}
}

func TestJavaLibraryOutputFilesRel(t *testing.T) {
	result := android.GroupFixturePreparers(
		PrepareForTestWithJavaDefaultModules,
	).RunTestWithBp(t, `
		java_library {
			name: "foo",
			srcs: ["a.java"],
		}

		java_import {
			name: "bar",
			jars: ["bar.aar"],

		}

		java_import {
			name: "baz",
			jars: ["baz.aar"],
			static_libs: ["bar"],
		}
	`)

	foo := result.ModuleForTests("foo", "android_common")
	bar := result.ModuleForTests("bar", "android_common")
	baz := result.ModuleForTests("baz", "android_common")

	fooOutputPaths := foo.OutputFiles(result.TestContext, t, "")
	barOutputPaths := bar.OutputFiles(result.TestContext, t, "")
	bazOutputPaths := baz.OutputFiles(result.TestContext, t, "")

	android.AssertPathsRelativeToTopEquals(t, "foo output path",
		[]string{"out/soong/.intermediates/foo/android_common/javac/foo.jar"}, fooOutputPaths)
	android.AssertPathsRelativeToTopEquals(t, "bar output path",
		[]string{"out/soong/.intermediates/bar/android_common/combined/bar.jar"}, barOutputPaths)
	android.AssertPathsRelativeToTopEquals(t, "baz output path",
		[]string{"out/soong/.intermediates/baz/android_common/combined/baz.jar"}, bazOutputPaths)

	android.AssertStringEquals(t, "foo relative output path",
		"foo.jar", fooOutputPaths[0].Rel())
	android.AssertStringEquals(t, "bar relative output path",
		"bar.jar", barOutputPaths[0].Rel())
	android.AssertStringEquals(t, "baz relative output path",
		"baz.jar", bazOutputPaths[0].Rel())
}

func TestCoverage(t *testing.T) {
	result := android.GroupFixturePreparers(
		PrepareForTestWithJavaDefaultModules,
		prepareForTestWithFrameworkJacocoInstrumentation,
		PrepareForTestWithTransitiveClasspathEnabled,
	).RunTestWithBp(t, `
		android_app {
			name: "foo",
			srcs: ["foo.java"],
			static_libs: ["android.car"],
			platform_apis: true,
		}

		// A library in InstrumentFrameworkModules
		java_library {
			name: "android.car",
			srcs: ["android.car.java"],
			installable: true,
		}
	`)

	foo := result.ModuleForTests("foo", "android_common")
	androidCar := result.ModuleForTests("android.car", "android_common")

	fooJacoco := foo.Rule("jacoco")
	fooCombine := foo.Description("for javac")

	androidCarJacoco := androidCar.Rule("jacoco")
	androidCarJavac := androidCar.Rule("javac")

	android.AssertStringEquals(t, "foo instrumentation rule inputs", fooJacoco.Input.String(), fooCombine.Output.String())
	android.AssertStringEquals(t, "android.car instrumentation rule inputs", androidCarJacoco.Input.String(), androidCarJavac.Output.String())

	// The input to instrumentation for the `foo` app contains the non-instrumented android.car classes.
	android.AssertStringListContains(t, "foo combined inputs", fooCombine.Inputs.Strings(), androidCarJavac.Output.String())
	android.AssertStringListDoesNotContain(t, "foo combined inputs", fooCombine.Inputs.Strings(), androidCarJacoco.Output.String())
}

func assertTestOnlyAndTopLevel(t *testing.T, ctx *android.TestResult, expectedTestOnly []string, expectedTopLevel []string) {
	t.Helper()
	actualTrueModules := []string{}
	actualTopLevelTests := []string{}
	addActuals := func(m blueprint.Module, key blueprint.ProviderKey[android.TestModuleInformation]) {
		if provider, ok := android.OtherModuleProvider(ctx.TestContext.OtherModuleProviderAdaptor(), m, key); ok {
			if provider.TestOnly {
				actualTrueModules = append(actualTrueModules, m.Name())
			}
			if provider.TopLevelTarget {
				actualTopLevelTests = append(actualTopLevelTests, m.Name())
			}
		}
	}

	ctx.VisitAllModules(func(m blueprint.Module) {
		addActuals(m, android.TestOnlyProviderKey)

	})

	notEqual, left, right := android.ListSetDifference(expectedTestOnly, actualTrueModules)
	if notEqual {
		t.Errorf("test-only: Expected but not found: %v, Found but not expected: %v", left, right)
	}

	notEqual, left, right = android.ListSetDifference(expectedTopLevel, actualTopLevelTests)
	if notEqual {
		t.Errorf("top-level: Expected but not found: %v, Found but not expected: %v", left, right)
	}
}

// Test that a dependency edge is created to the matching variant of a native library listed in `jni_libs` of java_binary
func TestNativeRequiredDepOfJavaBinary(t *testing.T) {
	findDepsOfModule := func(ctx *android.TestContext, module android.Module, depName string) []blueprint.Module {
		var ret []blueprint.Module
		ctx.VisitDirectDeps(module, func(dep blueprint.Module) {
			if dep.Name() == depName {
				ret = append(ret, dep)
			}
		})
		return ret
	}

	bp := cc.GatherRequiredDepsForTest(android.Android) + `
java_binary {
	name: "myjavabin",
	main_class: "com.android.MyJava",
	jni_libs: ["mynativelib"],
}
cc_library_shared {
	name: "mynativelib",
}
`
	res, _ := testJava(t, bp)
	deps := findDepsOfModule(res, res.ModuleForTests("myjavabin", "android_common").Module(), "mynativelib")
	android.AssertIntEquals(t, "Create a dep on the first variant", 1, len(deps))
}
