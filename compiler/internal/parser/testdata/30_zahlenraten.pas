program Zahlenraten;
uses Crt;
var
  Geheim, Tipp, Versuche: Integer;
  Taste: Char;
begin
  Randomize;
  repeat
    ClrScr;
    TextColor(Yellow);
    WriteLn('Ich denke mir eine Zahl zwischen 1 und 100.');
    NormVideo;
    Geheim := Random(100) + 1;
    Versuche := 0;
    repeat
      Write('Dein Tipp: ');
      ReadLn(Tipp);
      Inc(Versuche);
      if Tipp < Geheim then
        WriteLn('zu klein')
      else if Tipp > Geheim then
        WriteLn('zu groß')
    until Tipp = Geheim;
    TextColor(LightGreen + Blink);
    WriteLn('Richtig nach ', Versuche, ' Versuchen!');
    Delay(500);
    NormVideo;
    Write('Nochmal (j/n)? ');
    Taste := UpCase(ReadKey)
  until Taste <> 'J'
end.
