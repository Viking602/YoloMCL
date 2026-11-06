package modjar

import "fmt"

// Host faults (missing APIs, bytecode/resource limits and cancellation) are
// deliberately not Java exceptions and cannot be swallowed by a mod's catch.
type javaException struct {
	class, message string
	value          *object
	cause          *javaException
}

// Diagnostic text is not the Java exception's implementation-specific detail
// message. A mod must not branch on an invented getMessage() result.
type unboundExceptionMessage struct{ diagnostic string }

func (e javaException) Error() string { return e.diagnostic(0) }
func (e javaException) diagnostic(depth int) string {
	if depth >= 8 {
		return "exception cause limit"
	}
	message := e.message
	if message == "" && e.value != nil {
		switch data := e.value.data.(type) {
		case string:
			message = data
		case unboundExceptionMessage:
			message = data.diagnostic
		}
	}
	text := fmt.Sprintf("%s: %.512s", e.class, message)
	cause := e.cause
	if cause == nil && e.value != nil {
		if value, ok := e.value.fields["java/lang/Throwable.cause:Ljava/lang/Throwable;"].(*object); ok && value != nil && value != e.value {
			cause = &javaException{class: value.class, value: value}
		}
	}
	if cause != nil {
		text += "; caused by " + cause.diagnostic(depth+1)
	}
	return text
}
func raiseJava(class, message string) { panic(javaException{class: class, message: message}) }

var exceptionSupers = map[string]string{
	"java/util/ConcurrentModificationException":                            "java/lang/RuntimeException",
	"java/util/NoSuchElementException":                                     "java/lang/RuntimeException",
	"java/util/ServiceConfigurationError":                                  "java/lang/Error",
	"org/spongepowered/asm/mixin/injection/callback/CancellationException": "java/lang/RuntimeException",
	"java/lang/UnsupportedClassVersionError":                               "java/lang/ClassFormatError",
	"java/lang/ClassFormatError":                                           "java/lang/LinkageError",
	"java/lang/ClassNotFoundException":                                     "java/lang/ReflectiveOperationException",
	"java/lang/NoSuchFieldException":                                       "java/lang/ReflectiveOperationException",
	"java/lang/NoSuchMethodException":                                      "java/lang/ReflectiveOperationException",
	"java/lang/IllegalAccessException":                                     "java/lang/ReflectiveOperationException",
	"java/lang/InstantiationException":                                     "java/lang/ReflectiveOperationException",
	"java/lang/reflect/InvocationTargetException":                          "java/lang/ReflectiveOperationException",
	"java/lang/ReflectiveOperationException":                               "java/lang/Exception",
	"java/lang/NegativeArraySizeException":                                 "java/lang/RuntimeException",
	"java/util/EmptyStackException":                                        "java/lang/RuntimeException",
	"java/lang/UnsatisfiedLinkError":                                       "java/lang/LinkageError",
	"java/lang/LinkageError":                                               "java/lang/Error",
	"java/lang/ExceptionInInitializerError":                                "java/lang/LinkageError",
	"java/lang/NoClassDefFoundError":                                       "java/lang/LinkageError",
	"java/lang/Throwable":                                                  "java/lang/Object",
	"java/lang/Exception":                                                  "java/lang/Throwable",
	"java/lang/RuntimeException":                                           "java/lang/Exception",
	"java/lang/IllegalArgumentException":                                   "java/lang/RuntimeException",
	"java/lang/IllegalStateException":                                      "java/lang/RuntimeException",
	"java/lang/NumberFormatException":                                      "java/lang/IllegalArgumentException",
	"java/lang/NullPointerException":                                       "java/lang/RuntimeException",
	"java/lang/ArithmeticException":                                        "java/lang/RuntimeException",
	"java/lang/ClassCastException":                                         "java/lang/RuntimeException",
	"java/lang/IndexOutOfBoundsException":                                  "java/lang/RuntimeException",
	"java/lang/ArrayIndexOutOfBoundsException":                             "java/lang/IndexOutOfBoundsException",
	"java/lang/UnsupportedOperationException":                              "java/lang/RuntimeException",
	"java/lang/ArrayStoreException":                                        "java/lang/RuntimeException",
	"java/lang/Error":                                                      "java/lang/Throwable",
	"java/lang/AssertionError":                                             "java/lang/Error",
	"java/io/IOException":                                                  "java/lang/Exception",
	"java/io/FileNotFoundException":                                        "java/io/IOException",
	"java/io/UncheckedIOException":                                         "java/lang/RuntimeException",
	"java/nio/file/FileSystemException":                                    "java/io/IOException",
	"java/nio/file/DirectoryNotEmptyException":                             "java/nio/file/FileSystemException",
	"java/nio/file/NoSuchFileException":                                    "java/nio/file/FileSystemException",
	"java/nio/file/AccessDeniedException":                                  "java/nio/file/FileSystemException",
	"java/nio/file/FileAlreadyExistsException":                             "java/nio/file/FileSystemException",
	"java/io/UnsupportedEncodingException":                                 "java/io/IOException",
	"java/nio/charset/UnsupportedCharsetException":                         "java/lang/IllegalArgumentException",
	"java/nio/charset/CharacterCodingException":                            "java/io/IOException",
	"java/nio/charset/MalformedInputException":                             "java/nio/charset/CharacterCodingException",
}

func (v *machine) exceptionAssignable(e javaException, target string) bool {
	name := e.class
	for depth := 0; depth < 64 && name != ""; depth++ {
		if name == target {
			return true
		}
		if parent, ok := exceptionSupers[name]; ok {
			name = parent
			continue
		}
		if !v.fetchClass(name) {
			return false
		}
		name = v.load(name).super
	}
	return false
}
