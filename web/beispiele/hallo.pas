program Hallo;
{ Begruessung mit Eingabe: ReadLn wartet, bis eine Zeile mit Enter
  abgeschlossen ist. Eine leere Zeile beendet das Programm. }
uses Crt;
var
  Name: string;
begin
  TextColor(LightCyan);
  WriteLn('Hallo! Wie heisst du?');
  NormVideo;
  repeat
    Write('> ');
    ReadLn(Name);
    if Name <> '' then
    begin
      TextColor(Yellow);
      WriteLn('Guten Tag, ', Name, '! Dein Name hat ', Length(Name), ' Bytes.');
      NormVideo
    end
  until Name = '';
  WriteLn('Auf Wiedersehen.')
end.
