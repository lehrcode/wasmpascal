program Teile;
{ FSplit zerlegt rein als Zeichenkette; \ und / gelten beide als Trenner. }
uses Dos;

procedure Zeige(P: PathStr);
var
  D: DirStr;
  N: NameStr;
  E: ExtStr;
begin
  FSplit(P, D, N, E);
  WriteLn('[', D, '] [', N, '] [', E, ']')
end;

begin
  Zeige('C:\TP\PROG.PAS');
  Zeige('/daten/brief.txt');
  Zeige('notiz');
  Zeige('archiv.tar.gz');
  Zeige('a.b/datei');
  Zeige('.profil');
  Zeige('ordner/');
  Zeige('Größe/Übersicht.text');
  Zeige('')
end.
