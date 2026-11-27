program Attribute;
{ GetFAttr: nur Directory und Hidden (Name mit Punkt am Anfang) haben eine
  Wirkung; fehlende Datei 2, fehlendes Verzeichnis 3. }
uses Dos;
var
  F: Text;

procedure Zeige(N: string);
var
  G: file;
  A: Word;
begin
  A := 99;
  Assign(G, N);
  GetFAttr(G, A);
  WriteLn(N, ': ', A, ' DosError ', DosError)
end;

begin
  Assign(F, 'datei.txt'); Rewrite(F); Close(F);
  Assign(F, '.versteckt'); Rewrite(F); Close(F);
  MkDir('ordner');
  MkDir('.geheim');
  Zeige('datei.txt');
  Zeige('.versteckt');
  Zeige('ordner');
  Zeige('.geheim');
  Zeige('ordner/../datei.txt');
  Zeige('fehlt.txt');
  Zeige('fehlt/x.txt');
  WriteLn(ReadOnly, ' ', Hidden, ' ', SysFile, ' ', VolumeID, ' ', Directory, ' ', Archive, ' ', AnyFile)
end.
