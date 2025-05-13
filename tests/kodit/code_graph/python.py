# ruff: noqa
import os
from typing import List


def helper_function(x: List[str]) -> str:
    return " ".join(x)


class MyClass:
    def __init__(self, value: int):
        self.value = value

    def get_value(self) -> List[str]:
        return os.listdir()

    def print_value(self) -> None:
        print(self.value)


def main():
    obj = MyClass(42)
    result = helper_function(obj.get_value())
    return result
