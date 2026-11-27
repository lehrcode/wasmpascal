program Suche;
{ FSearch: zuerst das aktuelle Verzeichnis, dann die Liste (getrennt mit ;);
  nur Dateien zählen, keine Verzeichnisse. }
uses Dos;
var
  F: Text;
begin
  MkDir('bin');
  MkDir('lib');
  Assign(F, 'bin/werkzeug.exe'); Rewrite(F); Close(F);
  Assign(F, 'hier.txt'); Rewrite(F); Close(F);
  WriteLn('[', FSearch('hier.txt', 'bin;lib'), ']');
  WriteLn('[', FSearch('werkzeug.exe', 'lib;bin'), ']');
  WriteLn('[', FSearch('werkzeug.exe', 'lib;bin/'), ']');
  WriteLn('[', FSearch('werkzeug.exe', 'lib;bin\'), ']');
  WriteLn('[', FSearch('fehlt.txt', 'bin;lib'), ']');
  WriteLn('[', FSearch('lib', ''), ']');
  WriteLn('[', FSearch('werkzeug.exe', ''), ']')
end.
