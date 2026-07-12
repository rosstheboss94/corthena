from Cython.Build import cythonize
from setuptools import Extension, setup

setup(
    ext_modules=cythonize(
        [
            Extension(
                "corthena.cython_ext._compat",
                ["src/corthena/cython_ext/_compat.pyx"],
            )
        ],
        compiler_directives={"embedsignature": True, "language_level": "3"},
    ),
)
