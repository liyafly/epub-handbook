import sys
import os.path

# Make the project root importable so `import src.<module>` works.
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
